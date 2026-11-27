package ats

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// The adapters read board JSON the way the TypeScript did: untyped property
// access with ?? and || defaults, never a schema. Decoding into Go structs
// would fail a whole board on one field of an unexpected type, where the
// TypeScript carried on, so values stay as jsjson values (nil, bool,
// float64, string, []any, jsjson.Object) and these helpers mirror the
// JavaScript operators.

// undefinedT is JavaScript undefined: a property that is not there.
type undefinedT struct{}

var undefined any = undefinedT{}

// get is a chain of optional property reads, `v?.a?.b`. Anything that is not
// an object has no properties.
func get(v any, keys ...string) any {
	for _, k := range keys {
		o, ok := v.(jsjson.Object)
		if !ok {
			return undefined
		}
		val, present := o.Get(k)
		if !present {
			return undefined
		}
		v = val
	}
	return v
}

func nullish(v any) bool { return v == nil || v == undefined }

// truthy is JavaScript ToBoolean.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil, undefinedT:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case string:
		return x != ""
	default:
		return true
	}
}

// coalesce is `a ?? b ?? ...`: the first value that is not null or undefined.
func coalesce(vs ...any) any {
	for _, v := range vs {
		if !nullish(v) {
			return v
		}
	}
	return vs[len(vs)-1]
}

// or is `a || b || ...`: the first truthy value, else the last.
func or(vs ...any) any {
	for _, v := range vs {
		if truthy(v) {
			return v
		}
	}
	return vs[len(vs)-1]
}

// jsString is String(v).
func jsString(v any) string {
	switch x := v.(type) {
	case undefinedT:
		return "undefined"
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return string(jsjson.AppendNumber(nil, x))
	case string:
		return x
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if !nullish(e) {
				parts[i] = jsString(e)
			}
		}
		return strings.Join(parts, ",")
	case jsjson.Object:
		return "[object Object]"
	default:
		return fmt.Sprint(x)
	}
}

// text is a field the TypeScript typed as string and used as one. A string
// passes through, null or undefined is "", anything else is String(v).
func text(v any) string {
	if nullish(v) {
		return ""
	}
	return jsString(v)
}

// textOr is `v ?? def` for a string field.
func textOr(v any, def string) string {
	if nullish(v) {
		return def
	}
	return jsString(v)
}

func isString(v any) bool { _, ok := v.(string); return ok }

// array is `v ?? []` where v is expected to be an array; any other value
// counts as empty.
func array(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// merge is `{...a, ...b}` for two JSON objects; b's keys win, including
// nulls. A non-object side contributes nothing.
func merge(a, b any) any {
	ao, _ := a.(jsjson.Object)
	bo, _ := b.(jsjson.Object)
	out := make(jsjson.Object, 0, len(ao)+len(bo))
	idx := map[string]int{}
	add := func(o jsjson.Object) {
		for _, m := range jsjson.JSOrder(dedupe(o)) {
			if i, ok := idx[m.Key]; ok {
				out[i].Value = m.Value
				continue
			}
			idx[m.Key] = len(out)
			out = append(out, m)
		}
	}
	add(ao)
	add(bo)
	return jsjson.JSOrder(out)
}

// dedupe keeps the last value of a repeated key at its first position, as
// JSON.parse builds an object.
func dedupe(o jsjson.Object) jsjson.Object {
	seen := map[string]int{}
	out := make(jsjson.Object, 0, len(o))
	for _, m := range o {
		if i, ok := seen[m.Key]; ok {
			out[i].Value = m.Value
			continue
		}
		seen[m.Key] = len(out)
		out = append(out, m)
	}
	return out
}

// values is Object.values(v ?? {}) in JavaScript property order.
func values(v any) []any {
	o, ok := v.(jsjson.Object)
	if !ok {
		if s, ok := v.(string); ok {
			// Object.values on a string yields its characters.
			var out []any
			for _, u := range units(s) {
				out = append(out, unitsString([]uint16{u}))
			}
			return out
		}
		if a, ok := v.([]any); ok {
			return a
		}
		return nil
	}
	ordered := jsjson.JSOrder(dedupe(o))
	out := make([]any, len(ordered))
	for i, m := range ordered {
		out[i] = m.Value
	}
	return out
}

// parseJSONBody decodes a response body as Response.json() does: UTF-8 with
// replacement characters and a leading BOM dropped, then JSON.parse.
func parseJSONBody(body []byte) (any, error) {
	return jsjson.Parse(decodeBody(body))
}

// decodeBody is Response.text(): lossy UTF-8, BOM stripped.
func decodeBody(body []byte) string {
	s := jsjson.DecodeUTF8(body)
	return strings.TrimPrefix(s, "\uFEFF")
}

// units returns the UTF-16 code units of s. Lone surrogates kept by jsjson
// as WTF-8 (ED A0..BF xx) come back as themselves.
func units(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2] >= 0x80 && s[i+2] <= 0xBF {
			out = append(out, 0xD000|uint16(s[i+1]&0x3F)<<6|uint16(s[i+2]&0x3F))
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, 0xD800+uint16(r>>10), 0xDC00+uint16(r&0x3FF))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

// unitsString turns code units back into a Go string; a lone surrogate is
// written as WTF-8 so jsjson writes it back as the same \uXXXX escape.
func unitsString(u []uint16) string {
	var b strings.Builder
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c >= 0xD800 && c <= 0xDBFF && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] <= 0xDFFF {
			b.WriteRune(rune(c-0xD800)<<10 | rune(u[i+1]-0xDC00) + 0x10000)
			i++
			continue
		}
		if c >= 0xD800 && c <= 0xDFFF {
			b.WriteByte(0xE0 | byte(c>>12))
			b.WriteByte(0x80 | byte(c>>6&0x3F))
			b.WriteByte(0x80 | byte(c&0x3F))
			continue
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

// slice16 is s.slice(0, n) on a JavaScript string. It can split a surrogate
// pair, exactly as the TypeScript excerpt cut did.
func slice16(s string, n int) string {
	if len(s) <= n {
		// Every code unit takes at least one byte.
		return s
	}
	u := units(s)
	if len(u) <= n {
		return s
	}
	return unitsString(u[:n])
}

func len16(s string) int { return len(units(s)) }

// excerpt is `text ? text.slice(0, 400) : null`.
func excerpt(s string) *string {
	if s == "" {
		return nil
	}
	out := slice16(s, 400)
	return &out
}

// jsTrim is String.prototype.trim.
func jsTrim(s string) string { return htmlx.TrimJS(s) }

// encodeURIComponent escapes everything but A-Z a-z 0-9 - _ . ! ~ * ' ( ).
func encodeURIComponent(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}

// errURIMalformed is the URIError decodeURIComponent throws.
var errURIMalformed = errors.New("URI malformed")

// decodeURIComponent decodes every %XX escape; a broken escape or invalid
// UTF-8 is an error, as in JavaScript.
func decodeURIComponent(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	hexVal := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10
		}
		return -1
	}
	readByte := func(i int) (int, bool) {
		if i+2 >= len(s) || s[i] != '%' {
			return 0, false
		}
		h, l := hexVal(s[i+1]), hexVal(s[i+2])
		if h < 0 || l < 0 {
			return 0, false
		}
		return h<<4 | l, true
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '%' {
			b.WriteByte(s[i])
			i++
			continue
		}
		c, ok := readByte(i)
		if !ok {
			return "", errURIMalformed
		}
		i += 3
		if c < 0x80 {
			b.WriteByte(byte(c))
			continue
		}
		n := 0
		switch {
		case c&0xE0 == 0xC0:
			n = 2
		case c&0xF0 == 0xE0:
			n = 3
		case c&0xF8 == 0xF0:
			n = 4
		default:
			return "", errURIMalformed
		}
		buf := []byte{byte(c)}
		for k := 1; k < n; k++ {
			cc, ok := readByte(i)
			if !ok || cc&0xC0 != 0x80 {
				return "", errURIMalformed
			}
			buf = append(buf, byte(cc))
			i += 3
		}
		r, size := utf8.DecodeRune(buf)
		if r == utf8.RuneError || size != n {
			return "", errURIMalformed
		}
		b.Write(buf)
	}
	return b.String(), nil
}

// jsRegexp compiles a JavaScript regular expression source for Go: \s
// becomes JavaScript's Unicode whitespace class (Go's \s is ASCII only).
// Patterns that need lookaround, backreferences or counted repeats above
// 1000 are written as code instead.
func jsRegexp(src string) *regexp.Regexp {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src):
			if src[i+1] == 's' {
				if inClass {
					b.WriteString(htmlx.JSSpace)
				} else {
					b.WriteString("[" + htmlx.JSSpace + "]")
				}
			} else {
				b.WriteByte(c)
				b.WriteByte(src[i+1])
			}
			i++
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return regexp.MustCompile(b.String())
}

// filterJoin is `[a, b, ...].filter(Boolean).join(sep)` for values that are
// strings or absent.
func filterJoin(sep string, vs ...any) string {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		if truthy(v) {
			parts = append(parts, jsString(v))
		}
	}
	return strings.Join(parts, sep)
}

func containsStr(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

var remoteRe = regexp.MustCompile(`(?i)remote`)

// someRemote is `locations.some((l) => /remote/i.test(l))`.
func someRemote(locations []string) bool {
	for _, l := range locations {
		if remoteRe.MatchString(l) {
			return true
		}
	}
	return false
}

// jsObject builds an ordered object literal from key, value pairs.
func jsObject(kv ...any) jsjson.Object {
	o := make(jsjson.Object, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		o = append(o, jsjson.Member{Key: kv[i].(string), Value: kv[i+1]})
	}
	return o
}

// errorf is `new Error(message)`: a plain error whose text is the message.
func errorf(format string, args ...any) error { return fmt.Errorf(format, args...) }

func jsonOf(o jsjson.Object) []byte { return jsjson.Marshal(o) }

// strictEqual is === for JSON values: objects and arrays are equal only to
// themselves, which separately parsed values never are.
func strictEqual(a, b any) bool {
	switch x := a.(type) {
	case nil, undefinedT, bool, float64, string:
		switch b.(type) {
		case nil, undefinedT, bool, float64, string:
			return x == b
		}
	}
	return false
}
