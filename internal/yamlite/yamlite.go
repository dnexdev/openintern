// Package yamlite parses the small part of YAML that the registry files use
// (data/companies/*.yaml, data/curated/tier-1.yaml): block mappings and block
// sequences, nested by indentation, holding single-line plain, single-quoted
// or double-quoted scalars, with comments and blank lines.
//
// Everything else (flow collections, anchors and aliases, tags, block and
// multi-line scalars, several documents, complex keys) is an error that names
// the file and line, so a contributor learns at once what to change instead of
// getting a silently different value. Plain scalars resolve like the YAML 1.2
// core schema, which is what the TypeScript registry loader used: `123` is a
// number and `true` a boolean, so a numeric board token must be quoted.
package yamlite

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind is the type of a node after core-schema resolution.
type Kind int

const (
	Null Kind = iota
	String
	Int
	Float
	Bool
	Map
	Seq
)

// Pair is one entry of a mapping, in file order.
type Pair struct {
	Key   string
	Line  int
	Value *Node
}

// Node is a parsed value. Line is 1-based.
type Node struct {
	Kind  Kind
	Line  int
	Str   string  // String: the value; other scalars: the source text
	Int   int64   // Int
	Float float64 // Float (and Int, as a float)
	Bool  bool    // Bool
	Pairs []Pair  // Map
	Items []*Node // Seq
}

// TypeName names the node's type the way a JavaScript validator reports it
// ("string", "number", "boolean", "null", "array", "object"); a nil node is
// "undefined".
func (n *Node) TypeName() string {
	if n == nil {
		return "undefined"
	}
	switch n.Kind {
	case String:
		return "string"
	case Int, Float:
		return "number"
	case Bool:
		return "boolean"
	case Map:
		return "object"
	case Seq:
		return "array"
	}
	return "null"
}

// Get returns the value for key in a mapping, or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Map {
		return nil
	}
	for _, p := range n.Pairs {
		if p.Key == key {
			return p.Value
		}
	}
	return nil
}

// Error is a parse error at a line of a file.
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.File == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

type line struct {
	num    int
	indent int
	text   string // without indentation, comment-only lines dropped
}

type parser struct {
	file  string
	lines []line
	pos   int
}

func (p *parser) errf(lineNum int, format string, args ...any) error {
	return &Error{File: p.file, Line: lineNum, Msg: fmt.Sprintf(format, args...)}
}

// Parse parses src. file is used in error messages only. An empty document
// (only comments and blank lines) gives a Null node.
func Parse(file string, src []byte) (*Node, error) {
	p := &parser{file: file}
	if err := p.split(src); err != nil {
		return nil, err
	}
	if len(p.lines) == 0 {
		return &Node{Kind: Null, Line: 1}, nil
	}
	first := p.lines[0]
	if first.indent != 0 {
		return nil, p.errf(first.num, "the document must start at column 1")
	}
	n, err := p.block(0)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.lines) {
		l := p.lines[p.pos]
		return nil, p.errf(l.num, "unexpected content %q (check the indentation)", clip(l.text))
	}
	return n, nil
}

func (p *parser) split(src []byte) error {
	if !utf8.Valid(src) {
		for i, l := range strings.Split(string(src), "\n") {
			if !utf8.ValidString(l) {
				return p.errf(i+1, "invalid UTF-8")
			}
		}
	}
	text := strings.TrimPrefix(string(src), "\ufeff")
	raw := strings.Split(text, "\n")
	sawDocStart := false
	for i, l := range raw {
		num := i + 1
		l = strings.TrimSuffix(l, "\r")
		indent := 0
		for indent < len(l) && l[indent] == ' ' {
			indent++
		}
		rest := l[indent:]
		if strings.HasPrefix(rest, "\t") {
			if strings.TrimSpace(rest) == "" {
				continue
			}
			return p.errf(num, "tabs are not allowed for indentation; use spaces")
		}
		trimmed := strings.TrimRight(rest, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent == 0 && (trimmed == "---" || strings.HasPrefix(trimmed, "--- ")) {
			if sawDocStart || len(p.lines) > 0 {
				return p.errf(num, "only one YAML document per file is supported")
			}
			if strings.TrimSpace(trimmed[3:]) != "" && !strings.HasPrefix(strings.TrimSpace(trimmed[3:]), "#") {
				return p.errf(num, "content after --- is not supported; start it on the next line")
			}
			sawDocStart = true
			continue
		}
		if indent == 0 && (trimmed == "..." || strings.HasPrefix(trimmed, "%")) {
			return p.errf(num, "YAML directives and document end markers are not supported")
		}
		p.lines = append(p.lines, line{num: num, indent: indent, text: trimmed})
	}
	return nil
}

// block parses the node whose first line is the current one, at indent ind.
func (p *parser) block(ind int) (*Node, error) {
	l := p.lines[p.pos]
	if isSeqEntry(l.text) {
		return p.seq(ind, false)
	}
	return p.mapping(ind)
}

func isSeqEntry(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// seq parses a block sequence at indent ind. underKey is set for a list at
// the same indentation as its mapping key; a following key then ends it.
func (p *parser) seq(ind int, underKey bool) (*Node, error) {
	node := &Node{Kind: Seq, Line: p.lines[p.pos].num}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < ind {
			break
		}
		if l.indent > ind {
			return nil, p.errf(l.num, "unexpected indentation (multi-line values are not supported)")
		}
		if !isSeqEntry(l.text) {
			if underKey {
				break
			}
			if _, _, ok, _ := splitKey(l.text); ok {
				return nil, p.errf(l.num, "a mapping key cannot follow list items at the same indentation")
			}
			return nil, p.errf(l.num, "expected a list item (\"- ...\")")
		}
		if l.text == "-" {
			return nil, p.errf(l.num, "empty list items are not supported; put the value after \"- \"")
		}
		after := l.text[1:]
		spaces := len(after) - len(strings.TrimLeft(after, " "))
		rest := after[spaces:]
		col := ind + 1 + spaces
		if strings.HasPrefix(rest, "- ") || rest == "-" {
			return nil, p.errf(l.num, "nested lists on one line are not supported")
		}
		if strings.HasPrefix(rest, "#") {
			return nil, p.errf(l.num, "empty list items are not supported; put the value after \"- \"")
		}
		if _, _, ok, err := splitKey(rest); err != nil {
			return nil, p.errf(l.num, "%s", err)
		} else if ok {
			// "- key: value" starts a mapping whose keys sit at the column of "key".
			p.lines[p.pos] = line{num: l.num, indent: col, text: rest}
			item, err := p.mapping(col)
			if err != nil {
				return nil, err
			}
			node.Items = append(node.Items, item)
			continue
		}
		item, err := p.scalar(l.num, rest)
		if err != nil {
			return nil, err
		}
		node.Items = append(node.Items, item)
		p.pos++
	}
	return node, nil
}

func (p *parser) mapping(ind int) (*Node, error) {
	node := &Node{Kind: Map, Line: p.lines[p.pos].num}
	seen := map[string]int{}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < ind {
			break
		}
		if l.indent > ind {
			return nil, p.errf(l.num, "unexpected indentation (multi-line values are not supported)")
		}
		if isSeqEntry(l.text) {
			return nil, p.errf(l.num, "a list item cannot follow mapping keys at the same indentation")
		}
		key, rest, ok, err := splitKey(l.text)
		if err != nil {
			return nil, p.errf(l.num, "%s", err)
		}
		if !ok {
			// Report what the line is (a flow collection, a bad quote) when
			// it is not a plain "key: value".
			if _, serr := p.scalar(l.num, l.text); serr != nil {
				return nil, serr
			}
			return nil, p.errf(l.num, "expected \"key: value\", got %q", clip(l.text))
		}
		if prev, dup := seen[key]; dup {
			return nil, p.errf(l.num, "duplicate key %q (first at line %d)", key, prev)
		}
		seen[key] = l.num
		p.pos++
		var value *Node
		if rest != "" {
			value, err = p.scalar(l.num, rest)
			if err != nil {
				return nil, err
			}
		} else if p.pos < len(p.lines) && p.lines[p.pos].indent > ind {
			value, err = p.block(p.lines[p.pos].indent)
			if err != nil {
				return nil, err
			}
		} else if p.pos < len(p.lines) && p.lines[p.pos].indent == ind && isSeqEntry(p.lines[p.pos].text) {
			// A list may sit at the same indentation as its key.
			value, err = p.seq(ind, true)
			if err != nil {
				return nil, err
			}
		} else {
			value = &Node{Kind: Null, Line: l.num}
		}
		node.Pairs = append(node.Pairs, Pair{Key: key, Line: l.num, Value: value})
	}
	return node, nil
}

// splitKey splits "key: rest" (rest may be empty). ok is false when the text
// is not a mapping entry.
func splitKey(text string) (key, rest string, ok bool, err error) {
	if text == "" {
		return "", "", false, nil
	}
	if strings.ContainsRune("[]{}&*!|>%@`,", rune(text[0])) {
		return "", "", false, nil
	}
	switch text[0] {
	case '"', '\'':
		k, n, qerr := unquote(text)
		if qerr != nil {
			// Not a key; a quoted scalar is checked where it is used.
			return "", "", false, nil
		}
		after := text[n:]
		trimmed := strings.TrimLeft(after, " ")
		if !strings.HasPrefix(trimmed, ":") {
			return "", "", false, nil
		}
		value := trimmed[1:]
		if value != "" && value[0] != ' ' {
			return "", "", false, nil
		}
		return k, cleanValue(value), true, nil
	case '?':
		if text == "?" || strings.HasPrefix(text, "? ") {
			return "", "", false, fmt.Errorf("complex keys (\"? \") are not supported")
		}
	}
	idx := -1
	for i := 0; i < len(text); i++ {
		if text[i] == ':' && (i+1 == len(text) || text[i+1] == ' ') {
			idx = i
			break
		}
		if text[i] == '#' && i > 0 && text[i-1] == ' ' {
			break
		}
	}
	if idx <= 0 {
		return "", "", false, nil
	}
	key = strings.TrimRight(text[:idx], " ")
	return key, cleanValue(text[idx+1:]), true, nil
}

// cleanValue trims the value after "key:" and drops a whole-value comment.
func cleanValue(v string) string {
	v = strings.TrimLeft(v, " ")
	if strings.HasPrefix(v, "#") {
		return ""
	}
	return v
}

func (p *parser) scalar(lineNum int, text string) (*Node, error) {
	switch text[0] {
	case '"', '\'':
		s, n, err := unquote(text)
		if err != nil {
			return nil, p.errf(lineNum, "%s", err)
		}
		tail := strings.TrimLeft(text[n:], " ")
		if tail != "" && !(strings.HasPrefix(tail, "#") && n < len(text) && text[n] == ' ') {
			return nil, p.errf(lineNum, "unexpected text after the quoted value: %q", clip(tail))
		}
		return &Node{Kind: String, Line: lineNum, Str: s}, nil
	case '[', '{':
		return nil, p.errf(lineNum, "flow collections ([...] and {...}) are not supported; use one \"- item\" per line")
	case '&', '*':
		return nil, p.errf(lineNum, "anchors and aliases (& and *) are not supported")
	case '!':
		return nil, p.errf(lineNum, "tags (!) are not supported")
	case '|', '>':
		return nil, p.errf(lineNum, "block scalars (| and >) are not supported; keep the value on one line")
	case '%', '@', '`':
		return nil, p.errf(lineNum, "a plain value cannot start with %q; quote it", text[:1])
	case ',':
		return nil, p.errf(lineNum, "a plain value cannot start with \",\"; quote it")
	}
	if strings.HasPrefix(text, "- ") || text == "-" {
		return nil, p.errf(lineNum, "a list cannot start on the same line as its key")
	}
	if strings.HasPrefix(text, "? ") || strings.HasPrefix(text, ": ") {
		return nil, p.errf(lineNum, "complex keys are not supported")
	}
	// A plain scalar ends at " #" (a comment).
	if i := strings.Index(text, " #"); i >= 0 {
		text = strings.TrimRight(text[:i], " ")
	}
	if strings.Contains(text, ": ") || strings.HasSuffix(text, ":") {
		return nil, p.errf(lineNum, "nested mappings on one line are not supported; quote the value if it contains \": \"")
	}
	return resolvePlain(lineNum, text), nil
}

var (
	reInt   = regexp.MustCompile(`^[-+]?[0-9]+$`)
	reOct   = regexp.MustCompile(`^0o[0-7]+$`)
	reHex   = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	reFloat = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?$`)
)

// resolvePlain applies the YAML 1.2 core schema to a plain scalar.
func resolvePlain(lineNum int, s string) *Node {
	n := &Node{Line: lineNum, Str: s}
	switch s {
	case "", "~", "null", "Null", "NULL":
		n.Kind = Null
		return n
	case "true", "True", "TRUE":
		n.Kind, n.Bool = Bool, true
		return n
	case "false", "False", "FALSE":
		n.Kind = Bool
		return n
	case ".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF":
		n.Kind, n.Float = Float, math.Inf(1)
		return n
	case "-.inf", "-.Inf", "-.INF":
		n.Kind, n.Float = Float, math.Inf(-1)
		return n
	case ".nan", ".NaN", ".NAN":
		n.Kind, n.Float = Float, math.NaN()
		return n
	}
	switch {
	case reInt.MatchString(s):
		n.Kind = Int
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			n.Int, n.Float = v, float64(v)
		} else {
			n.Kind = Float
			n.Float, _ = strconv.ParseFloat(s, 64)
		}
	case reOct.MatchString(s):
		n.Kind = Int
		v, _ := strconv.ParseUint(s[2:], 8, 64)
		n.Int, n.Float = int64(v), float64(v)
	case reHex.MatchString(s):
		n.Kind = Int
		v, _ := strconv.ParseUint(s[2:], 16, 64)
		n.Int, n.Float = int64(v), float64(v)
	case reFloat.MatchString(s):
		n.Kind = Float
		n.Float, _ = strconv.ParseFloat(strings.TrimSuffix(s, "."), 64)
	default:
		n.Kind = String
	}
	return n
}

// unquote reads a single- or double-quoted scalar at the start of text and
// returns its value and the number of bytes it used.
func unquote(text string) (string, int, error) {
	q := text[0]
	var b strings.Builder
	for i := 1; i < len(text); i++ {
		c := text[i]
		if q == '\'' {
			if c == '\'' {
				if i+1 < len(text) && text[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				return b.String(), i + 1, nil
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			if i+1 >= len(text) {
				return "", 0, fmt.Errorf("unterminated escape in a double-quoted value")
			}
			i++
			e := text[i]
			switch e {
			case '0':
				b.WriteByte(0)
			case 'a':
				b.WriteByte('\a')
			case 'b':
				b.WriteByte('\b')
			case 't', '\t':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'v':
				b.WriteByte('\v')
			case 'f':
				b.WriteByte('\f')
			case 'r':
				b.WriteByte('\r')
			case 'e':
				b.WriteByte(0x1b)
			case ' ':
				b.WriteByte(' ')
			case '"', '/', '\\':
				b.WriteByte(e)
			case 'N':
				b.WriteString("\u0085")
			case '_':
				b.WriteString("\u00a0")
			case 'L':
				b.WriteString("\u2028")
			case 'P':
				b.WriteString("\u2029")
			case 'x', 'u', 'U':
				n := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
				if i+1+n > len(text) {
					return "", 0, fmt.Errorf("short \\%c escape in a double-quoted value", e)
				}
				v, err := strconv.ParseUint(text[i+1:i+1+n], 16, 32)
				if err != nil || !utf8.ValidRune(rune(v)) {
					return "", 0, fmt.Errorf("bad \\%c escape in a double-quoted value", e)
				}
				b.WriteRune(rune(v))
				i += n
			default:
				return "", 0, fmt.Errorf("unknown escape \\%c in a double-quoted value", e)
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, fmt.Errorf("unterminated quoted value (multi-line quoted values are not supported)")
}

func clip(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}
