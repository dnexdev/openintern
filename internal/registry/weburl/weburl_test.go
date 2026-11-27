package weburl

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/vectors.json is `new URL(input)` in Node 26 for each input
// (regenerate with /tmp/openintern-pipeline/url-vectors.ts, kept in the
// pipeline status file).
func TestVectorsMatchNode(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Input    string `json:"input"`
		OK       bool   `json:"ok"`
		Protocol string `json:"protocol"`
		Username string `json:"username"`
		Password string `json:"password"`
		Hostname string `json:"hostname"`
		Port     string `json:"port"`
		Pathname string `json:"pathname"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		u, err := Parse(v.Input)
		if !v.OK {
			if err == nil {
				t.Errorf("%q: Node rejects it, Go parsed %+v", v.Input, *u)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: Node parses it, Go: %v", v.Input, err)
			continue
		}
		want := URL{v.Protocol, v.Username, v.Password, v.Hostname, v.Port, v.Pathname}
		if *u != want {
			t.Errorf("%q:\n got %+v\nwant %+v", v.Input, *u, want)
		}
	}
}
