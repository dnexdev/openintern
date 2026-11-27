package safedate

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testdata/date_parse_node.json is Date.parse in Node 26 for each input,
// run with TZ set to each zone (null for NaN).
func TestDateParseMatchesNode(t *testing.T) {
	raw, err := os.ReadFile("testdata/date_parse_node.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors map[string][][2]any
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for zone, rows := range vectors {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatalf("zone %s: %v", zone, err)
		}
		for _, row := range rows {
			in := row[0].(string)
			got, ok := dateParseIn(in, loc)
			want, wantOK := row[1].(float64)
			if ok != wantOK || (ok && got != want) {
				t.Errorf("%s: Date.parse(%q) = %v,%v want %v", zone, in, int64(got), ok, row[1])
			}
		}
	}
}
