package ats

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/registry/weburl"
)

// jsURL is the subset of JavaScript's URL the adapters use: parse, resolve
// against a base, read the parts, edit searchParams and the host, and
// serialize. Scheme, host and path parsing is weburl's (the WHATWG parser);
// the query and fragment are percent-encoded here with the standard's sets.
type jsURL struct {
	weburl.URL
	// query is the serialized query without "?"; hasQuery separates "?" from none.
	query    string
	hasQuery bool
	fragment string
	hasFrag  bool
}

var errInvalidURL = errors.New("Invalid URL")

func preprocessURL(input string) string {
	input = strings.TrimFunc(input, func(r rune) bool { return r <= 0x20 })
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, input)
}

// parseURL is `new URL(input)`.
func parseURL(input string) (*jsURL, error) {
	input = preprocessURL(input)
	u := &jsURL{}
	if i := strings.IndexByte(input, '#'); i >= 0 {
		u.fragment = percentEncodeSet(input[i+1:], fragmentSet)
		u.hasFrag = true
		input = input[:i]
	}
	special := false
	if c := strings.IndexByte(input, ':'); c > 0 {
		switch strings.ToLower(input[:c]) {
		case "http", "https", "ws", "wss", "ftp", "file":
			special = true
		}
	}
	if i := strings.IndexByte(input, '?'); i >= 0 {
		set := querySet
		if special {
			set = specialQuerySet
		}
		u.query = percentEncodeSet(input[i+1:], set)
		u.hasQuery = true
		input = input[:i]
	}
	base, err := weburl.Parse(input)
	if err != nil {
		return nil, errInvalidURL
	}
	u.URL = *base
	return u, nil
}

var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// resolveURL is `new URL(ref, base)` for the reference forms careers pages
// use: absolute, scheme-relative, path-absolute, query, fragment and
// path-relative.
func resolveURL(ref string, base *jsURL) (*jsURL, error) {
	ref = preprocessURL(ref)
	if schemeRe.MatchString(ref) {
		return parseURL(ref)
	}
	host := base.Hostname
	if base.Port != "" {
		host += ":" + base.Port
	}
	prefix := base.Protocol + "//" + host
	switch {
	case strings.HasPrefix(ref, "//") || strings.HasPrefix(ref, `\\`):
		return parseURL(base.Protocol + ref)
	case strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, `\`):
		return parseURL(prefix + ref)
	case ref == "":
		out := *base
		out.fragment, out.hasFrag = "", false
		return &out, nil
	case strings.HasPrefix(ref, "?"):
		return parseURL(prefix + base.Pathname + ref)
	case strings.HasPrefix(ref, "#"):
		return parseURL(prefix + base.Pathname + base.search() + ref)
	default:
		dir := base.Pathname
		if i := strings.LastIndexByte(dir, '/'); i >= 0 {
			dir = dir[:i+1]
		} else {
			dir = "/"
		}
		return parseURL(prefix + dir + ref)
	}
}

func (u *jsURL) search() string {
	if !u.hasQuery || u.query == "" {
		return ""
	}
	return "?" + u.query
}

func (u *jsURL) host() string {
	if u.Port != "" {
		return u.Hostname + ":" + u.Port
	}
	return u.Hostname
}

// origin is URL.origin for http(s) URLs.
func (u *jsURL) origin() string {
	switch u.Protocol {
	case "http:", "https:", "ws:", "wss:", "ftp:":
		return u.Protocol + "//" + u.host()
	}
	return "null"
}

// String is URL.href.
func (u *jsURL) String() string {
	var b strings.Builder
	b.WriteString(u.Protocol)
	if u.Hostname != "" || u.Protocol == "file:" || isSpecialProtocol(u.Protocol) {
		b.WriteString("//")
		if u.Username != "" || u.Password != "" {
			b.WriteString(u.Username)
			if u.Password != "" {
				b.WriteString(":" + u.Password)
			}
			b.WriteString("@")
		}
		b.WriteString(u.host())
	}
	b.WriteString(u.Pathname)
	if u.hasQuery {
		b.WriteString("?" + u.query)
	}
	if u.hasFrag {
		b.WriteString("#" + u.fragment)
	}
	return b.String()
}

func isSpecialProtocol(p string) bool {
	switch p {
	case "http:", "https:", "ws:", "wss:", "ftp:", "file:":
		return true
	}
	return false
}

// setHost is the URL.host setter: the host part is replaced if it parses,
// then a port after ":" is applied if it is valid. As in the URL standard,
// a bad port leaves the new host in place and the old port unchanged.
func (u *jsURL) setHost(h string) {
	if i := strings.IndexAny(h, "/?#\\"); i >= 0 {
		h = h[:i]
	}
	hostPart, portPart, hasPort := strings.Cut(h, ":")
	if hostPart == "" {
		return
	}
	probe, err := weburl.Parse(u.Protocol + "//" + hostPart + "/")
	if err != nil || probe.Hostname == "" {
		return
	}
	u.Hostname = probe.Hostname
	if !hasPort || portPart == "" {
		return
	}
	n := 0
	for i := 0; i < len(portPart); i++ {
		c := portPart[i]
		if c < '0' || c > '9' {
			return
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return
		}
	}
	port := strconv.Itoa(n)
	if (u.Protocol == "https:" && port == "443") || (u.Protocol == "http:" && port == "80") {
		port = ""
	}
	u.Port = port
}

// setPathname is the URL.pathname setter for an absolute path.
func (u *jsURL) setPathname(p string) {
	probe, err := weburl.Parse(u.Protocol + "//" + u.host() + p)
	if err == nil {
		u.Pathname = probe.Pathname
	}
}

type param struct{ name, value string }

// params is URLSearchParams parsing of the current query.
func (u *jsURL) params() []param {
	var out []param
	if !u.hasQuery || u.query == "" {
		return nil
	}
	for _, seq := range strings.Split(u.query, "&") {
		if seq == "" {
			continue
		}
		name, value, _ := strings.Cut(seq, "=")
		out = append(out, param{formDecode(name), formDecode(value)})
	}
	return out
}

func (u *jsURL) hasParam(name string) bool {
	for _, p := range u.params() {
		if p.name == name {
			return true
		}
	}
	return false
}

// setParam is searchParams.set: replace the first pair named name and drop
// the rest, or append; the query is then re-serialized.
func (u *jsURL) setParam(name, value string) {
	ps := u.params()
	out := ps[:0:0]
	found := false
	for _, p := range ps {
		if p.name == name {
			if !found {
				out = append(out, param{name, value})
				found = true
			}
			continue
		}
		out = append(out, p)
	}
	if !found {
		out = append(out, param{name, value})
	}
	var b strings.Builder
	for i, p := range out {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(formEncode(p.name))
		b.WriteByte('=')
		b.WriteString(formEncode(p.value))
	}
	u.query = b.String()
	u.hasQuery = u.query != ""
}

// formEncode is the application/x-www-form-urlencoded serializer.
func formEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '*', c == '-', c == '.', c == '_':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}

// formDecode is the urlencoded parser's name/value decoding: + is a space,
// %XX is a byte, the result is read as UTF-8 with replacement.
func formDecode(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	return decodeBody(percentDecodeBytes(s))
}

func percentDecodeBytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
			out = append(out, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 2
			continue
		}
		out = append(out, s[i])
	}
	return out
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

type byteSet func(c byte) bool

func c0Control(c byte) bool       { return c < 0x20 || c > 0x7E }
func fragmentSet(c byte) bool     { return c0Control(c) || strings.IndexByte(" \"<>`", c) >= 0 }
func querySet(c byte) bool        { return c0Control(c) || strings.IndexByte(" \"#<>", c) >= 0 }
func specialQuerySet(c byte) bool { return querySet(c) || c == '\'' }

func percentEncodeSet(s string, set byteSet) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if set(c) {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
