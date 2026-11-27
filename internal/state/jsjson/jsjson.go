// Package jsjson reads and writes JSON exactly as JavaScript's JSON.parse and
// JSON.stringify do, so files the TypeScript pipeline wrote (state, dumps)
// round-trip byte for byte.
//
// encoding/json differs from JSON.stringify in ways that matter here: it
// escapes <, >, & and U+2028/U+2029, writes nil slices as null, and cannot
// represent a lone UTF-16 surrogate (it decodes "\ud83d" to U+FFFD). Lone
// surrogates are real: the TypeScript excerpt cut `text.slice(0, 399)` counts
// UTF-16 code units and can split an emoji, and JSON.stringify then writes
// "\ud83d". This package keeps such a surrogate in a Go string as its
// three-byte generalized UTF-8 form (WTF-8, bytes ED A0..BF xx) and writes it
// back as the same escape.
//
// Values are plain Go values: nil, bool, float64 (every JSON number), string,
// []any and Object (members in source order). The encoder also accepts int,
// int64, []string, []int and *Object for convenience.
package jsjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Member is one key of an object, in order.
type Member struct {
	Key   string
	Value any
}

// Object is a JSON object with its keys in order.
type Object []Member

// Get returns the value of key and whether it is present (the last one wins,
// as in JSON.parse).
func (o Object) Get(key string) (any, bool) {
	for i := len(o) - 1; i >= 0; i-- {
		if o[i].Key == key {
			return o[i].Value, true
		}
	}
	return nil, false
}

// Marshal is JSON.stringify(v).
func Marshal(v any) []byte {
	var e encoder
	e.value(v, 0)
	return e.buf
}

// MarshalIndent is JSON.stringify(v, null, indent).
func MarshalIndent(v any, indent string) []byte {
	e := encoder{indent: indent}
	e.value(v, 0)
	return e.buf
}

type encoder struct {
	buf    []byte
	indent string
}

func (e *encoder) newline(level int) {
	if e.indent == "" {
		return
	}
	e.buf = append(e.buf, '\n')
	for i := 0; i < level; i++ {
		e.buf = append(e.buf, e.indent...)
	}
}

func (e *encoder) value(v any, level int) {
	switch x := v.(type) {
	case nil:
		e.buf = append(e.buf, "null"...)
	case bool:
		e.buf = strconv.AppendBool(e.buf, x)
	case string:
		e.buf = AppendString(e.buf, x)
	case int:
		e.buf = strconv.AppendInt(e.buf, int64(x), 10)
	case int64:
		e.buf = strconv.AppendInt(e.buf, x, 10)
	case float64:
		e.buf = AppendNumber(e.buf, x)
	case []string:
		e.array(len(x), level, func(i int) { e.buf = AppendString(e.buf, x[i]) })
	case []int:
		e.array(len(x), level, func(i int) { e.buf = strconv.AppendInt(e.buf, int64(x[i]), 10) })
	case []any:
		e.array(len(x), level, func(i int) { e.value(x[i], level+1) })
	case Object:
		e.object(x, level)
	case *Object:
		if x == nil {
			e.buf = append(e.buf, "null"...)
			return
		}
		e.object(*x, level)
	default:
		panic(fmt.Sprintf("jsjson: unsupported type %T", v))
	}
}

func (e *encoder) array(n, level int, item func(i int)) {
	if n == 0 {
		e.buf = append(e.buf, "[]"...)
		return
	}
	e.buf = append(e.buf, '[')
	for i := 0; i < n; i++ {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.newline(level + 1)
		item(i)
	}
	e.newline(level)
	e.buf = append(e.buf, ']')
}

func (e *encoder) object(o Object, level int) {
	if len(o) == 0 {
		e.buf = append(e.buf, "{}"...)
		return
	}
	e.buf = append(e.buf, '{')
	for i, m := range o {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.newline(level + 1)
		e.buf = AppendString(e.buf, m.Key)
		e.buf = append(e.buf, ':')
		if e.indent != "" {
			e.buf = append(e.buf, ' ')
		}
		e.value(m.Value, level+1)
	}
	e.newline(level)
	e.buf = append(e.buf, '}')
}

// AppendNumber writes a number as JavaScript's Number#toString does inside
// JSON.stringify (NaN and infinities become null, -0 becomes 0).
func AppendNumber(dst []byte, f float64) []byte {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		return append(dst, "null"...)
	case f == 0:
		return append(dst, '0')
	case f == math.Trunc(f) && math.Abs(f) < 1e21:
		return strconv.AppendFloat(dst, f, 'f', -1, 64)
	}
	// encoding/json formats float64 with the ECMAScript algorithm.
	out, _ := json.Marshal(f)
	return append(dst, out...)
}

const hexDigits = "0123456789abcdef"

// AppendString writes s quoted as JSON.stringify does: only ", \ and control
// characters are escaped (\b \t \n \f \r by name, others as \u00xx), lone
// surrogates (WTF-8) as \udxxx, everything else raw, including <, >, & and
// U+2028/U+2029. Bytes that are neither UTF-8 nor WTF-8 are written as U+FFFD.
func AppendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch c {
			case '"', '\\':
				dst = append(dst, '\\', c)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\t':
				dst = append(dst, '\\', 't')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\r':
				dst = append(dst, '\\', 'r')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&15])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != utf8.RuneError || size != 1 {
			i += size
			continue
		}
		dst = append(dst, s[start:i]...)
		if hi, ok := surrogateAt(s, i); ok {
			// A WTF-8 pair (high then low) is one character to JavaScript.
			if lo, ok2 := surrogateAt(s, i+3); ok2 && utf16.IsSurrogate(rune(hi)) && hi < 0xdc00 && lo >= 0xdc00 {
				dst = utf8.AppendRune(dst, utf16.DecodeRune(rune(hi), rune(lo)))
				i += 6
			} else {
				dst = append(dst, '\\', 'u', hexDigits[hi>>12], hexDigits[hi>>8&15], hexDigits[hi>>4&15], hexDigits[hi&15])
				i += 3
			}
		} else {
			dst = append(dst, "\uFFFD"...)
			i++
		}
		start = i
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// surrogateAt decodes a WTF-8 surrogate (ED A0..BF 80..BF) at s[i:].
func surrogateAt(s string, i int) (uint16, bool) {
	if i+2 >= len(s) || s[i] != 0xed || s[i+1] < 0xa0 || s[i+1] > 0xbf || s[i+2] < 0x80 || s[i+2] > 0xbf {
		return 0, false
	}
	return uint16(0xd000 | uint16(s[i+1]&0x3f)<<6 | uint16(s[i+2]&0x3f)), true
}

// appendSurrogate writes a lone surrogate as WTF-8.
func appendSurrogate(dst []byte, u uint16) []byte {
	return append(dst, 0xed, 0x80|byte(u>>6&0x3f), 0x80|byte(u&0x3f))
}

// CompareCodeUnits orders strings by UTF-16 code units, like JavaScript's
// < on strings (the TypeScript byCodeUnit). It differs from Go's byte order
// only when a character above U+FFFF meets one in U+E000..U+FFFF.
func CompareCodeUnits(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] < utf8.RuneSelf && b[j] < utf8.RuneSelf {
			if a[i] != b[j] {
				if a[i] < b[j] {
					return -1
				}
				return 1
			}
			i++
			j++
			continue
		}
		ua, na := unitsAt(a, i)
		ub, nb := unitsAt(b, j)
		if ua != ub {
			if ua[0] != ub[0] {
				if ua[0] < ub[0] {
					return -1
				}
				return 1
			}
			if ua[1] < ub[1] {
				return -1
			}
			return 1
		}
		i += na
		j += nb
	}
	switch {
	case len(a)-i == len(b)-j:
		return 0
	case i < len(a):
		return 1
	}
	return -1
}

// unitsAt returns the UTF-16 code units of the character at s[i:] (second
// unit 0 when there is one) and its length in bytes.
func unitsAt(s string, i int) ([2]uint16, int) {
	if u, ok := surrogateAt(s, i); ok {
		return [2]uint16{u, 0}, 3
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r >= 0x10000 {
		hi, lo := utf16.EncodeRune(r)
		return [2]uint16{uint16(hi), uint16(lo)}, size
	}
	return [2]uint16{uint16(r), 0}, size
}

// Len16 is the string's length in UTF-16 code units (JavaScript .length).
func Len16(s string) int {
	n := 0
	for i := 0; i < len(s); {
		u, size := unitsAt(s, i)
		n++
		if u[1] != 0 {
			n++
		}
		i += size
	}
	return n
}

// DecodeUTF8 decodes bytes the way Node reads a file as "utf8" (the WHATWG
// decoder): each maximal invalid subsequence becomes one U+FFFD. Valid input
// is returned unchanged without copying more than once.
func DecodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var out bytes.Buffer
	out.Grow(len(b) + 16)
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out.WriteByte(c)
			i++
			continue
		}
		need, lower, upper := 0, byte(0x80), byte(0xbf)
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c >= 0xe0 && c <= 0xef:
			need = 2
			if c == 0xe0 {
				lower = 0xa0
			} else if c == 0xed {
				upper = 0x9f
			}
		case c >= 0xf0 && c <= 0xf4:
			need = 3
			if c == 0xf0 {
				lower = 0x90
			} else if c == 0xf4 {
				upper = 0x8f
			}
		default:
			out.WriteString("\uFFFD")
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lower || b[j] > upper {
				ok = false
				break
			}
			lower, upper = 0x80, 0xbf
			j++
		}
		if !ok {
			out.WriteString("\uFFFD")
			i = j
			continue
		}
		out.Write(b[i:j])
		i = j
	}
	return out.String()
}

// JSOrder puts array-index keys ("0", "17") first in ascending order, then
// the rest in insertion order: the property order of a JavaScript object
// built by assigning the members in turn.
func JSOrder(o Object) Object { return jsKeyOrder(o) }

func jsKeyOrder(o Object) Object {
	var idx []Member
	for _, m := range o {
		if _, ok := arrayIndex(m.Key); ok {
			idx = append(idx, m)
		}
	}
	if len(idx) == 0 {
		return o
	}
	sort.SliceStable(idx, func(a, b int) bool {
		x, _ := arrayIndex(idx[a].Key)
		y, _ := arrayIndex(idx[b].Key)
		return x < y
	})
	out := append(Object{}, idx...)
	for _, m := range o {
		if _, ok := arrayIndex(m.Key); !ok {
			out = append(out, m)
		}
	}
	return out
}

func arrayIndex(k string) (uint64, bool) {
	if k == "" || len(k) > 10 || (k[0] == '0' && len(k) > 1) {
		return 0, false
	}
	for i := 0; i < len(k); i++ {
		if k[i] < '0' || k[i] > '9' {
			return 0, false
		}
	}
	v, _ := strconv.ParseUint(k, 10, 64)
	return v, v < 1<<32-1
}

// SyntaxError is a JSON.parse failure.
type SyntaxError struct {
	Offset int
	Msg    string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("%s at offset %d", e.Msg, e.Offset) }

// Parse is JSON.parse for a document that is already a Go string (decode
// bytes from disk with DecodeUTF8 first). Objects come back as Object,
// arrays as []any, numbers as float64.
func Parse(s string) (any, error) {
	p := parser{s: s}
	p.space()
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, p.fail("unexpected data after the value")
	}
	return v, nil
}

type parser struct {
	s string
	i int
}

const maxDepth = 512

func (p *parser) fail(msg string) error { return &SyntaxError{Offset: p.i, Msg: msg} }

func (p *parser) space() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) value(depth int) (any, error) {
	if depth > maxDepth {
		return nil, p.fail("nesting too deep")
	}
	if p.i >= len(p.s) {
		return nil, p.fail("unexpected end of input")
	}
	switch c := p.s[p.i]; {
	case c == '{':
		return p.object(depth)
	case c == '[':
		return p.array(depth)
	case c == '"':
		return p.str()
	case c == 't':
		return p.literal("true", true)
	case c == 'f':
		return p.literal("false", false)
	case c == 'n':
		return p.literal("null", nil)
	case c == '-' || c >= '0' && c <= '9':
		return p.number()
	}
	return nil, p.fail("unexpected character")
}

func (p *parser) literal(word string, v any) (any, error) {
	if len(p.s)-p.i < len(word) || p.s[p.i:p.i+len(word)] != word {
		return nil, p.fail("invalid literal")
	}
	p.i += len(word)
	return v, nil
}

func (p *parser) number() (any, error) {
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	digits := func() int {
		n := 0
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			n++
		}
		return n
	}
	if p.i < len(p.s) && p.s[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return nil, p.fail("invalid number")
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		if digits() == 0 {
			return nil, p.fail("invalid number")
		}
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			return nil, p.fail("invalid number")
		}
	}
	f, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		// Out of range: JSON.parse gives ±Infinity, which writes as null.
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f, nil
		}
		return nil, p.fail("invalid number")
	}
	return f, nil
}

func (p *parser) str() (string, error) {
	p.i++ // opening quote
	start := p.i
	// Fast path: no escapes.
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '"' {
			s := p.s[start:p.i]
			p.i++
			return s, nil
		}
		if c == '\\' {
			break
		}
		if c < 0x20 {
			return "", p.fail("control character in string")
		}
		p.i++
	}
	buf := []byte(p.s[start:p.i])
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return string(buf), nil
		case c < 0x20:
			return "", p.fail("control character in string")
		case c != '\\':
			buf = append(buf, c)
			p.i++
			continue
		}
		p.i++
		if p.i >= len(p.s) {
			return "", p.fail("unterminated string")
		}
		e := p.s[p.i]
		p.i++
		switch e {
		case '"', '\\', '/':
			buf = append(buf, e)
		case 'b':
			buf = append(buf, '\b')
		case 'f':
			buf = append(buf, '\f')
		case 'n':
			buf = append(buf, '\n')
		case 'r':
			buf = append(buf, '\r')
		case 't':
			buf = append(buf, '\t')
		case 'u':
			u, ok := p.hex4()
			if !ok {
				return "", p.fail("invalid \\u escape")
			}
			switch {
			case u >= 0xd800 && u < 0xdc00:
				// A high surrogate pairs with an escaped low surrogate right after it.
				if p.i+6 <= len(p.s) && p.s[p.i] == '\\' && p.s[p.i+1] == 'u' {
					save := p.i
					p.i += 2
					if lo, ok := p.hex4(); ok && lo >= 0xdc00 && lo < 0xe000 {
						buf = utf8.AppendRune(buf, utf16.DecodeRune(rune(u), rune(lo)))
						continue
					}
					p.i = save
				}
				buf = appendSurrogate(buf, u)
			case u >= 0xdc00 && u < 0xe000:
				buf = appendSurrogate(buf, u)
			default:
				buf = utf8.AppendRune(buf, rune(u))
			}
		default:
			return "", p.fail("invalid escape")
		}
	}
	return "", p.fail("unterminated string")
}

func (p *parser) hex4() (uint16, bool) {
	if p.i+4 > len(p.s) {
		return 0, false
	}
	v, err := strconv.ParseUint(p.s[p.i:p.i+4], 16, 16)
	if err != nil {
		return 0, false
	}
	p.i += 4
	return uint16(v), true
}

func (p *parser) array(depth int) ([]any, error) {
	p.i++
	out := []any{}
	p.space()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return out, nil
	}
	for {
		p.space()
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		p.space()
		if p.i >= len(p.s) {
			return nil, p.fail("unterminated array")
		}
		switch p.s[p.i] {
		case ',':
			p.i++
		case ']':
			p.i++
			return out, nil
		default:
			return nil, p.fail("expected , or ]")
		}
	}
}

func (p *parser) object(depth int) (Object, error) {
	p.i++
	out := Object{}
	p.space()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return out, nil
	}
	for {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return nil, p.fail("expected a string key")
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return nil, p.fail("expected :")
		}
		p.i++
		p.space()
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		// JSON.parse keeps the first position of a repeated key and the last value.
		dup := false
		for i := range out {
			if out[i].Key == k {
				out[i].Value = v
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, Member{Key: k, Value: v})
		}
		p.space()
		if p.i >= len(p.s) {
			return nil, p.fail("unterminated object")
		}
		switch p.s[p.i] {
		case ',':
			p.i++
		case '}':
			p.i++
			return jsKeyOrder(out), nil
		default:
			return nil, p.fail("expected , or }")
		}
	}
}

// EncodeUTF8 is how Node writes a string to a file as UTF-8: a lone
// surrogate (kept in Go strings as WTF-8) becomes U+FFFD.
func EncodeUTF8(s string) string {
	var out []byte
	last := 0
	for i := 0; i < len(s); i++ {
		if _, ok := surrogateAt(s, i); ok {
			out = append(append(out, s[last:i]...), "�"...)
			i += 2
			last = i + 1
		}
	}
	if out == nil {
		return s
	}
	return string(append(out, s[last:]...))
}
