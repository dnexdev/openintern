package jsjson

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// testdata/vectors.json comes from Node 26 (JSON.parse, JSON.stringify,
// string < and .length); see /tmp/openintern-pipeline/jsjson-vectors.ts in
// the pipeline status file.
type vectorFile struct {
	Vectors []struct {
		In, Compact, Pretty string
	} `json:"vectors"`
	Bad     []string `json:"bad"`
	Order   []string `json:"order"`
	Lengths [][2]any `json:"lengths"`
}

func loadVectors(t *testing.T) vectorFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectorFile
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func b64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoundTripMatchesNode(t *testing.T) {
	for _, v := range loadVectors(t).Vectors {
		in := b64(t, v.In)
		val, err := Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got, want := string(Marshal(val)), b64(t, v.Compact); got != want {
			t.Errorf("compact %q\n got %q\nwant %q", in, got, want)
		}
		if got, want := string(MarshalIndent(val, "  ")), b64(t, v.Pretty); got != want {
			t.Errorf("pretty %q\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestRejectsWhatJSONParseRejects(t *testing.T) {
	for _, s := range loadVectors(t).Bad {
		if v, err := Parse(s); err == nil {
			t.Errorf("%q parsed as %#v", s, v)
		}
	}
}

func TestCompareCodeUnitsAndLength(t *testing.T) {
	v := loadVectors(t)
	var strs []string
	for _, enc := range v.Order {
		s, err := Parse(enc)
		if err != nil {
			t.Fatal(err)
		}
		strs = append(strs, s.(string))
	}
	shuffled := append([]string(nil), strs...)
	sort.Slice(shuffled, func(i, j int) bool { return shuffled[i] > shuffled[j] })
	sort.SliceStable(shuffled, func(i, j int) bool { return CompareCodeUnits(shuffled[i], shuffled[j]) < 0 })
	for i := range strs {
		if shuffled[i] != strs[i] {
			t.Fatalf("order differs at %d: got %q, want %q", i, shuffled[i], strs[i])
		}
	}
	for _, l := range v.Lengths {
		s, err := Parse(l[0].(string))
		if err != nil {
			t.Fatal(err)
		}
		if got := Len16(s.(string)); got != int(l[1].(float64)) {
			t.Errorf("Len16(%s) = %d, want %v", l[0], got, l[1])
		}
	}
}

func TestDecodeUTF8LikeNode(t *testing.T) {
	cases := map[string]string{
		"plain":                        "plain",
		"a\xffb":                       "a�b",
		"\xe2\x82a":                    "�a", // truncated 3-byte sequence: one replacement
		"\xed\xa0\xbd":                 "���",
		"\xf0\x9f\x98":                 "�",
		"\xc0\xaf":                     "��",
		"\xf4\x90\x80\x80":             "����",
		"ok \xf0\x9f\x98\x80 \xe2\x80": "ok \U0001F600 �",
	}
	for in, want := range cases {
		if got := DecodeUTF8([]byte(in)); got != want {
			t.Errorf("DecodeUTF8(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEncodeConveniences(t *testing.T) {
	o := Object{
		{"s", []string{}}, {"i", []int{1, 2}}, {"n", nil}, {"o", &Object{{"k", int64(3)}}}, {"e", Object{}},
	}
	if got, want := string(Marshal(o)), `{"s":[],"i":[1,2],"n":null,"o":{"k":3},"e":{}}`; got != want {
		t.Errorf("got %s want %s", got, want)
	}
	want := "{\n  \"s\": [],\n  \"i\": [\n    1,\n    2\n  ],\n  \"n\": null,\n  \"o\": {\n    \"k\": 3\n  },\n  \"e\": {}\n}"
	if got := string(MarshalIndent(o, "  ")); got != want {
		t.Errorf("got %s want %s", got, want)
	}
}
