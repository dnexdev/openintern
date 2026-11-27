package state

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/localecompare.json is Node 26's a.localeCompare(b) over every
// pair of a set of timestamps, slugs and punctuation/case variants.
func TestLocaleCompareMatchesNode(t *testing.T) {
	b, err := os.ReadFile("testdata/localecompare.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][3]any
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		a, bb, want := c[0].(string), c[1].(string), int(c[2].(float64))
		if got := LocaleCompare(a, bb); got != want {
			t.Errorf("LocaleCompare(%q, %q) = %d, want %d", a, bb, got, want)
		}
	}
}
