package htmlx

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{`Foo &amp;quot;Bar&amp;quot; <b>Baz</b>`, `Foo &quot;Bar&quot; Baz`},
		{`Foo &quot;Bar&quot;`, `Foo "Bar"`},
		{"Before</script foo>After", "Before After"},
		{"<p>Hello&nbsp;world</p><br/>&amp; more", "Hello world & more"},
		{"🔥 Meta", "Meta"},
		{"  🇺🇸🎓 hello", "hello"},
		{"a  b\n c", "a b c"},
		{"<br>x<br/>y", "x y"},
		{"🔒passport", "passport"},
		{"hello 🔥", "hello 🔥"},
		{"&#039;hi&#39;", "'hi'"},
		{`<div class="a">Z</div>`, "Z"},
	}
	for _, tc := range cases {
		if got := Strip(tc.in); got != tc.want {
			t.Errorf("Strip(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := StripTags("<p>Hello&nbsp;world</p><br/>&amp; more"); got != "Hello world & more" {
		t.Fatalf("StripTags = %q", got)
	}
	if got := StripTags("<p>Write code</p>"); got != "Write code" {
		t.Fatalf("StripTags paragraph = %q", got)
	}
}

// testdata/strip_node.json is [input, stripHtml, stripHtmlTags] from the
// TypeScript helpers on crafted inputs and dump titles and excerpts.
func TestStripMatchesTypeScript(t *testing.T) {
	raw, err := os.ReadFile("testdata/strip_node.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][3]string
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if got := Strip(r[0]); got != r[1] {
			t.Errorf("Strip(%q) = %q, want %q", r[0], got, r[1])
		}
		if got := StripTags(r[0]); got != r[2] {
			t.Errorf("StripTags(%q) = %q, want %q", r[0], got, r[2])
		}
	}
}
