package tools

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestLocaleCompareMatchesNode(t *testing.T) {
	raw, err := os.ReadFile("testdata/collate_node.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ Input, Sorted []string }
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	got := slices.Clone(v.Input)
	slices.SortStableFunc(got, localeCompare)
	bad := 0
	for i := range got {
		if got[i] != v.Sorted[i] {
			if bad < 15 {
				t.Errorf("position %d: got %q, Node %q", i, got[i], v.Sorted[i])
			}
			bad++
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d positions differ", bad, len(got))
	}
}
