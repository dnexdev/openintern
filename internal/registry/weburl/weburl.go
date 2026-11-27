// Package weburl parses absolute URLs the way the WHATWG URL Standard (and so
// JavaScript's `new URL`) does, for the parts OpenIntern decides on: whether
// a string is a URL at all, its scheme, credentials, hostname, port and path.
//
// net/url is not enough for an allowlist: it keeps "/acme/../evil/" as is,
// reads "https://host:443/" as having a port, and treats "\" as an ordinary
// character, while a browser normalizes all three. A check that disagrees
// with the browser about where a link goes is no check, so this follows the
// standard's parser state by state for special schemes (http, https, ws, wss,
// ftp, file). Queries and fragments are skipped, not normalized.
//
// Known gaps, all of which only change how an already-rejected host is
// spelled: internationalized hosts are mapped with NFKC and lowercasing
// rather than the full UTS #46 table, and IPv6 hosts are serialized by
// net/netip (which differs for IPv4-mapped addresses).
package weburl

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/unicodex"
)

// URL holds the parsed components, spelled like the JavaScript URL
// properties of the same names.
type URL struct {
	Protocol string // scheme plus ":", lowercase ("https:")
	Username string // percent-encoded, as URL.username
	Password string
	Hostname string // lowercase ASCII for domains; IPv6 in brackets
	Port     string // "" when absent or the scheme's default
	Pathname string
}

// ErrInvalid is returned for input that `new URL` would reject.
var ErrInvalid = errors.New("invalid URL")

var defaultPorts = map[string]string{
	"http": "80", "https": "443", "ws": "80", "wss": "443", "ftp": "21", "file": "",
}

func isSpecial(scheme string) bool {
	_, ok := defaultPorts[scheme]
	return ok
}

// CanParse reports whether `new URL(input)` would succeed.
func CanParse(input string) bool {
	_, err := Parse(input)
	return err == nil
}

// Parse parses an absolute URL.
func Parse(input string) (*URL, error) {
	// Leading and trailing C0 controls and spaces are dropped; tabs and
	// newlines are removed everywhere.
	input = strings.TrimFunc(input, func(r rune) bool { return r <= 0x20 })
	if strings.ContainsAny(input, "\t\n\r") {
		input = strings.Map(func(r rune) rune {
			if r == '\t' || r == '\n' || r == '\r' {
				return -1
			}
			return r
		}, input)
	}
	colon := strings.IndexByte(input, ':')
	if colon <= 0 || !validScheme(input[:colon]) {
		return nil, ErrInvalid
	}
	scheme := strings.ToLower(input[:colon])
	rest := input[colon+1:]
	u := &URL{Protocol: scheme + ":"}

	if scheme == "file" {
		return parseFile(u, rest)
	}
	if !isSpecial(scheme) {
		if !strings.HasPrefix(rest, "//") {
			// Opaque path ("mailto:x", "javascript:..."): always parses.
			u.Pathname = opaquePath(rest)
			return u, nil
		}
		return parseAuthorityAndPath(u, scheme, rest[2:], false)
	}
	// Special schemes skip any run of slashes and backslashes before the authority.
	rest = strings.TrimLeft(rest, "/\\")
	return parseAuthorityAndPath(u, scheme, rest, true)
}

func validScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return s != ""
}

func parseAuthorityAndPath(u *URL, scheme, rest string, special bool) (*URL, error) {
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '/' || c == '?' || c == '#' || (special && c == '\\') {
			end = i
			break
		}
	}
	authority, tail := rest[:end], rest[end:]
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		userinfo := authority[:at]
		authority = authority[at+1:]
		user, pass, _ := strings.Cut(userinfo, ":")
		u.Username = percentEncode(user, userinfoSet)
		u.Password = percentEncode(pass, userinfoSet)
		if authority == "" {
			return nil, ErrInvalid
		}
	}
	hostPart, portPart := authority, ""
	hasPort := false
	if strings.HasPrefix(authority, "[") {
		close := strings.IndexByte(authority, ']')
		if close < 0 {
			return nil, ErrInvalid
		}
		hostPart = authority[:close+1]
		after := authority[close+1:]
		if after != "" {
			if after[0] != ':' {
				return nil, ErrInvalid
			}
			portPart, hasPort = after[1:], true
		}
	} else if i := strings.IndexByte(authority, ':'); i >= 0 {
		hostPart, portPart, hasPort = authority[:i], authority[i+1:], true
	}
	if hostPart == "" {
		if special || hasPort || u.Username != "" || u.Password != "" {
			return nil, ErrInvalid
		}
	} else {
		host, err := parseHost(hostPart, special)
		if err != nil {
			return nil, err
		}
		u.Hostname = host
	}
	if hasPort && portPart != "" {
		for i := 0; i < len(portPart); i++ {
			if portPart[i] < '0' || portPart[i] > '9' {
				return nil, ErrInvalid
			}
		}
		trimmed := strings.TrimLeft(portPart, "0")
		if len(trimmed) > 5 {
			return nil, ErrInvalid
		}
		n, _ := strconv.Atoi("0" + trimmed)
		if n > 65535 {
			return nil, ErrInvalid
		}
		if p := strconv.Itoa(n); p != defaultPorts[scheme] || !special {
			u.Port = p
		}
	}
	u.Pathname = path(tail, special)
	return u, nil
}

func parseFile(u *URL, rest string) (*URL, error) {
	// file: URLs never carry credentials or ports; only the host can fail.
	if strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, `\\`) || strings.HasPrefix(rest, `/\`) || strings.HasPrefix(rest, `\/`) {
		rest = rest[2:]
		end := len(rest)
		for i := 0; i < len(rest); i++ {
			if c := rest[i]; c == '/' || c == '\\' || c == '?' || c == '#' {
				end = i
				break
			}
		}
		if host := rest[:end]; host != "" && !isWindowsDriveLetter(host) {
			h, err := parseHost(host, true)
			if err != nil {
				return nil, err
			}
			if h != "localhost" {
				u.Hostname = h
			}
			rest = rest[end:]
		}
	}
	u.Pathname = path(rest, true)
	return u, nil
}

func isWindowsDriveLetter(s string) bool {
	return len(s) == 2 && (s[0]|0x20 >= 'a' && s[0]|0x20 <= 'z') && (s[1] == ':' || s[1] == '|')
}

// path serializes the path that starts tail (up to "?" or "#"), resolving
// "." and ".." segments (including their %2e spellings) as browsers do.
func path(tail string, special bool) string {
	if i := strings.IndexAny(tail, "?#"); i >= 0 {
		tail = tail[:i]
	}
	if special {
		tail = strings.ReplaceAll(tail, `\`, "/")
	}
	if tail == "" {
		if special {
			return "/"
		}
		return ""
	}
	segs := strings.Split(strings.TrimPrefix(tail, "/"), "/")
	var out []string
	for i, seg := range segs {
		last := i == len(segs)-1
		switch {
		case isDoubleDot(seg):
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			if last {
				out = append(out, "")
			}
		case isSingleDot(seg):
			if last {
				out = append(out, "")
			}
		default:
			out = append(out, percentEncode(seg, pathSet))
		}
	}
	return "/" + strings.Join(out, "/")
}

func isSingleDot(s string) bool {
	return s == "." || strings.EqualFold(s, "%2e")
}

func isDoubleDot(s string) bool {
	switch strings.ToLower(s) {
	case "..", ".%2e", "%2e.", "%2e%2e":
		return true
	}
	return false
}

func opaquePath(s string) string {
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	return percentEncode(s, c0Set)
}

type encodeSet func(c byte) bool

func c0Set(c byte) bool       { return c < 0x20 || c > 0x7e }
func pathSet(c byte) bool     { return c0Set(c) || strings.IndexByte(" \"#<>?`{}", c) >= 0 }
func userinfoSet(c byte) bool { return pathSet(c) || strings.IndexByte("/:;=@[\\]^|", c) >= 0 }

func percentEncode(s string, set encodeSet) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if set(c) {
			b.WriteByte('%')
			b.WriteByte("0123456789ABCDEF"[c>>4])
			b.WriteByte("0123456789ABCDEF"[c&15])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func percentDecode(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			v, _ := strconv.ParseUint(s[i+1:i+3], 16, 8)
			b.WriteByte(byte(v))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

const forbiddenHost = "\x00\t\n\r #/:<>?@[\\]^|"

func parseHost(input string, special bool) (string, error) {
	if strings.HasPrefix(input, "[") {
		if !strings.HasSuffix(input, "]") {
			return "", ErrInvalid
		}
		addr, err := netip.ParseAddr(input[1 : len(input)-1])
		if err != nil || !addr.Is6() || addr.Zone() != "" {
			return "", ErrInvalid
		}
		return "[" + addr.String() + "]", nil
	}
	if !special {
		// Opaque host: forbidden code points fail, the rest is kept encoded.
		if strings.ContainsAny(input, forbiddenHost) {
			return "", ErrInvalid
		}
		return percentEncode(input, c0Set), nil
	}
	domain := percentDecode(input)
	if !utf8.ValidString(domain) {
		return "", ErrInvalid
	}
	ascii, err := domainToASCII(domain)
	if err != nil {
		return "", err
	}
	for i := 0; i < len(ascii); i++ {
		c := ascii[i]
		if c < 0x20 || c == 0x7f || strings.IndexByte(forbiddenHost+"%", c) >= 0 {
			return "", ErrInvalid
		}
	}
	if endsInNumber(ascii) {
		return parseIPv4(ascii)
	}
	return ascii, nil
}

// domainToASCII lowercases ASCII and maps the rest with NFKC and lowercasing
// before Punycode, an approximation of UTS #46 processing.
func domainToASCII(domain string) (string, error) {
	ascii := true
	for i := 0; i < len(domain); i++ {
		if domain[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(domain), nil
	}
	mapped := strings.ToLower(unicodex.NFKC(strings.ToLower(domain)))
	// Ideographic and fullwidth full stops separate labels too.
	mapped = strings.NewReplacer("。", ".", "．", ".", "｡", ".").Replace(mapped)
	labels := strings.Split(mapped, ".")
	for i, l := range labels {
		if isASCII(l) {
			continue
		}
		enc, err := punycodeEncode(l)
		if err != nil {
			return "", err
		}
		labels[i] = "xn--" + enc
	}
	return strings.Join(labels, "."), nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func endsInNumber(host string) bool {
	parts := strings.Split(host, ".")
	if parts[len(parts)-1] == "" {
		if len(parts) == 1 {
			return false
		}
		parts = parts[:len(parts)-1]
	}
	last := parts[len(parts)-1]
	if last != "" && strings.Trim(last, "0123456789") == "" {
		return true
	}
	_, ok := ipv4Number(last)
	return ok
}

func ipv4Number(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	base := 10
	switch {
	case len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X"):
		s, base = s[2:], 16
	case len(s) >= 2 && s[0] == '0':
		s, base = s[1:], 8
	}
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		// Too large still counts as a number (it then fails the range check).
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return 1 << 63, true
		}
		return 0, false
	}
	return v, true
}

func parseIPv4(host string) (string, error) {
	parts := strings.Split(host, ".")
	if parts[len(parts)-1] == "" && len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 4 {
		return "", ErrInvalid
	}
	nums := make([]uint64, len(parts))
	for i, p := range parts {
		n, ok := ipv4Number(p)
		if !ok {
			return "", ErrInvalid
		}
		nums[i] = n
	}
	for _, n := range nums[:len(nums)-1] {
		if n > 255 {
			return "", ErrInvalid
		}
	}
	last := nums[len(nums)-1]
	if last >= 1<<(8*(5-len(nums))) {
		return "", ErrInvalid
	}
	addr := last
	for i, n := range nums[:len(nums)-1] {
		addr += n << (8 * (3 - i))
	}
	return strconv.FormatUint(addr>>24, 10) + "." + strconv.FormatUint(addr>>16&255, 10) + "." +
		strconv.FormatUint(addr>>8&255, 10) + "." + strconv.FormatUint(addr&255, 10), nil
}

// punycodeEncode is RFC 3492 encoding of one label.
func punycodeEncode(label string) (string, error) {
	const (
		base, tmin, tmax, skew, damp = 36, 1, 26, 38, 700
		initialBias, initialN        = 72, 128
	)
	runes := []rune(label)
	var out []byte
	for _, r := range runes {
		if r < 0x80 {
			out = append(out, byte(r))
		}
	}
	b := len(out)
	h := b
	if b > 0 {
		out = append(out, '-')
	}
	n, delta, bias := rune(initialN), 0, initialBias
	digit := func(d int) byte {
		if d < 26 {
			return byte('a' + d)
		}
		return byte('0' + d - 26)
	}
	adapt := func(delta, numPoints int, first bool) int {
		if first {
			delta /= damp
		} else {
			delta /= 2
		}
		delta += delta / numPoints
		k := 0
		for delta > ((base-tmin)*tmax)/2 {
			delta /= base - tmin
			k += base
		}
		return k + (base-tmin+1)*delta/(delta+skew)
	}
	for h < len(runes) {
		m := rune(0x7fffffff)
		for _, r := range runes {
			if r >= n && r < m {
				m = r
			}
		}
		if int(m-n) > (1<<31-1-delta)/(h+1) {
			return "", ErrInvalid
		}
		delta += int(m-n) * (h + 1)
		n = m
		for _, r := range runes {
			if r < n {
				delta++
			}
			if r == n {
				q := delta
				for k := base; ; k += base {
					t := k - bias
					if t < tmin {
						t = tmin
					} else if t > tmax {
						t = tmax
					}
					if q < t {
						break
					}
					out = append(out, digit(t+(q-t)%(base-t)))
					q = (q - t) / (base - t)
				}
				out = append(out, digit(q))
				bias = adapt(delta, h+1, h == b)
				delta = 0
				h++
			}
		}
		delta++
		n++
	}
	return string(out), nil
}
