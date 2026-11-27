package yamlite

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// toJSON renders a node the way JSON.stringify renders what the TypeScript
// yaml package parsed (keys in file order, Infinity and NaN as null).
func toJSON(n *Node) string {
	var b strings.Builder
	var walk func(n *Node)
	walk = func(n *Node) {
		switch n.Kind {
		case Null:
			b.WriteString("null")
		case Bool:
			b.WriteString(strconv.FormatBool(n.Bool))
		case Int, Float:
			if math.IsInf(n.Float, 0) || math.IsNaN(n.Float) {
				b.WriteString("null")
				return
			}
			out, _ := json.Marshal(n.Float)
			b.Write(out)
		case String:
			writeString(&b, n.Str)
		case Seq:
			b.WriteByte('[')
			for i, it := range n.Items {
				if i > 0 {
					b.WriteByte(',')
				}
				walk(it)
			}
			b.WriteByte(']')
		case Map:
			b.WriteByte('{')
			for i, p := range n.Pairs {
				if i > 0 {
					b.WriteByte(',')
				}
				writeString(&b, p.Key)
				b.WriteByte(':')
				walk(p.Value)
			}
			b.WriteByte('}')
		}
	}
	walk(n)
	return b.String()
}

func writeString(b *strings.Builder, s string) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b.WriteString(strings.TrimSuffix(buf.String(), "\n"))
}

func TestFeaturesMatchTypeScript(t *testing.T) {
	src, err := os.ReadFile("testdata/features.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/features.json")
	if err != nil {
		t.Fatal(err)
	}
	n, err := Parse("features.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	if got := toJSON(n); got != strings.TrimSpace(string(want)) {
		t.Errorf("mismatch\n got: %s\nwant: %s", got, want)
	}
}

// Every registry file in the repository parses. data_test.go in
// internal/registry checks the values against the schema.
func TestRepositoryDataParses(t *testing.T) {
	files, _ := filepath.Glob("../../data/companies/*.yaml")
	files = append(files, "../../data/curated/tier-1.yaml")
	if len(files) < 2 {
		t.Fatalf("no data files found")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		n, err := Parse(f, src)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if n.Kind != Map || len(n.Pairs) != 1 {
			t.Errorf("%s: want one top-level key, got %s", f, toJSON(n)[:40])
		}
	}
}

func TestMalformed(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
		msg       string
	}{
		{"tab indent", "companies:\n\t- a\n", 2, "tabs are not allowed"},
		{"flow seq", "companies: [a, b]\n", 1, "flow collections"},
		{"flow map", "companies:\n  - {name: a}\n", 2, "flow collections"},
		{"anchor", "companies:\n  - name: &x a\n", 2, "anchors and aliases"},
		{"alias", "companies:\n  - name: *x\n", 2, "anchors and aliases"},
		{"tag", "slugs:\n  - !!str 1\n", 2, "tags"},
		{"block scalar", "companies:\n  - name: |\n      a\n", 2, "block scalars"},
		{"folded scalar", "companies:\n  - name: >\n      a\n", 2, "block scalars"},
		{"multi-line plain", "companies:\n  - name: a\n      b\n", 3, "multi-line values"},
		{"multi-line quoted", "companies:\n  - name: \"a\n      b\"\n", 2, "unterminated quoted value"},
		{"two documents", "a: 1\n---\nb: 2\n", 2, "only one YAML document"},
		{"complex key", "? a\n: b\n", 1, "complex keys"},
		{"duplicate key", "companies:\n  - name: a\n    name: b\n", 3, "duplicate key \"name\""},
		{"duplicate top key", "slugs:\n  - a\nslugs:\n  - b\n", 3, "duplicate key \"slugs\""},
		{"nested map on one line", "companies:\n  - name: a: b\n", 2, "nested mappings on one line"},
		{"bad indentation", "companies:\n  - name: a\n   slug: b\n", 3, "unexpected indentation"},
		{"seq after map", "a: 1\n- b\n", 2, "a list item cannot follow mapping keys"},
		{"key after seq", "a:\n  - b\n  c: 1\n", 3, "a mapping key cannot follow list items"},
		{"missing colon space", "companies:\n  name:a\n", 2, "expected \"key: value\""},
		{"empty item", "slugs:\n  -\n", 2, "empty list items"},
		{"comment item", "slugs:\n  - # x\n", 2, "empty list items"},
		{"text after quote", "slugs:\n  - \"a\" b\n", 2, "unexpected text after the quoted value"},
		{"bad escape", "slugs:\n  - \"a\\q\"\n", 2, "unknown escape"},
		{"short escape", "slugs:\n  - \"\\u12\"\n", 2, "escape"},
		{"seq on key line", "slugs: - a\n", 1, "a list cannot start on the same line"},
		{"indented start", "  slugs:\n  - a\n", 1, "must start at column 1"},
		{"trailing garbage", "slugs:\n  - a\n  - b\n x\n", 4, "unexpected indentation"},
		{"flow at top", "{a: 1}\n", 1, "flow collections"},
		{"directive", "%YAML 1.2\n---\na: 1\n", 1, "directives"},
		{"invalid utf8", "slugs:\n  - \xff\n", 2, "invalid UTF-8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := Parse("x.yaml", []byte(c.src))
			if err == nil {
				t.Fatalf("no error, got %s", toJSON(n))
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("error %T %v is not *Error", err, err)
			}
			if e.Line != c.line || !strings.Contains(e.Msg, c.msg) {
				t.Errorf("got %q, want line %d containing %q", err, c.line, c.msg)
			}
			if !strings.HasPrefix(err.Error(), "x.yaml:") {
				t.Errorf("error %q does not name the file", err)
			}
		})
	}
}

func TestScalarsAndShapes(t *testing.T) {
	cases := []struct{ src, want string }{
		{"", "null"},
		{"# only a comment\n", "null"},
		{"a:\n", `{"a":null}`},
		{"a: 'x' # c\n", `{"a":"x"}`},
		{"a: \"x\"   # c\n", `{"a":"x"}`},
		{"a:   # c\n  - b\n", `{"a":["b"]}`},
		{"a:\r\n  - b\r\n", `{"a":["b"]}`},
		{"\ufeffa: 1\n", `{"a":1}`},
		{"a:\n    -   x\n    -   y: 1\n        z: 2\n", `{"a":["x",{"y":1,"z":2}]}`},
		{"a:\n  b:\n    c: d\n", `{"a":{"b":{"c":"d"}}}`},
		{"a: k:1\n", `{"a":"k:1"}`},
		{"a: TRUE\nb: tRue\nc: NULL\nd: 012\ne: +5\nf: 1_000\n", `{"a":true,"b":"tRue","c":null,"d":12,"e":5,"f":"1_000"}`},
	}
	for _, c := range cases {
		n, err := Parse("", []byte(c.src))
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if got := toJSON(n); got != c.want {
			t.Errorf("%q: got %s, want %s", c.src, got, c.want)
		}
	}
}

func TestTypeName(t *testing.T) {
	n, err := Parse("", []byte("a: 1\nb: x\nc: true\nd:\ne:\n  - 1\nf:\n  g: 1\nh: 1.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "number", "b": "string", "c": "boolean", "d": "null", "e": "array", "f": "object", "h": "number", "zz": "undefined"}
	for k, w := range want {
		if got := n.Get(k).TypeName(); got != w {
			t.Errorf("%s: got %s, want %s", k, got, w)
		}
	}
	if n.Get("e").Line != 6 || n.Get("e").Items[0].Line != 6 {
		t.Errorf("lines: %d %d", n.Get("e").Line, n.Get("e").Items[0].Line)
	}
}

// TestTypeScriptGolden compares against the TypeScript yaml package on any
// set of files. Generate the golden lines ("path<TAB>JSON") with
// /tmp/openintern-pipeline/yaml2json.ts and point YAMLITE_TS_GOLDEN at them.
func TestTypeScriptGolden(t *testing.T) {
	golden := os.Getenv("YAMLITE_TS_GOLDEN")
	if golden == "" {
		t.Skip("YAMLITE_TS_GOLDEN not set")
	}
	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		path, want, _ := strings.Cut(l, "\t")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		node, err := Parse(path, src)
		if strings.HasPrefix(want, "ERROR ") {
			if err == nil {
				t.Errorf("%s: TypeScript failed (%s), Go parsed it", path, want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if got := toJSON(node); got != want {
			t.Errorf("%s differs\n got: %.300s\nwant: %.300s", path, got, want)
		}
		n++
	}
	t.Logf("%d files identical", n)
}
