package jstime

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testdata/vectors.json: Node 26 Date.parse, toISOString and state.ts
// pgTimestamp (generated with TZ=UTC; no vector depends on the zone).
func TestVectorsMatchNode(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Parse [][2]any `json:"parse"`
		ISO   [][3]any `json:"iso"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, c := range v.Parse {
		s := c[0].(string)
		ms, ok := Parse(s)
		switch want := c[1].(type) {
		case nil:
			if ok {
				t.Errorf("Parse(%q) = %d, want NaN", s, ms)
			}
		case float64:
			if !ok || ms != int64(want) {
				t.Errorf("Parse(%q) = %d %v, want %d", s, ms, ok, int64(want))
			}
		}
	}
	for _, c := range v.ISO {
		ms := int64(c[0].(float64))
		if got := MustISO(ms); got != c[1] {
			t.Errorf("ISO(%d) = %s, want %s", ms, got, c[1])
		}
		if got := PgTimestamp(ms); got != c[2] {
			t.Errorf("PgTimestamp(%d) = %s, want %s", ms, got, c[2])
		}
	}
	if _, ok := ISO(MaxMS + 1); ok {
		t.Error("ISO out of range must fail like toISOString throws")
	}
}

// state.test.ts: new rows use Postgres's row_to_json timestamp format.
func TestPgTimestamp(t *testing.T) {
	cases := map[string]string{
		"2026-11-21T10:00:00.500Z": "2026-11-21T10:00:00.5+00:00",
		"2026-11-21T10:00:00.000Z": "2026-11-21T10:00:00+00:00",
		"2026-11-21T10:00:00.010Z": "2026-11-21T10:00:00.01+00:00",
		"2026-11-21T10:00:00.123Z": "2026-11-21T10:00:00.123+00:00",
	}
	for in, want := range cases {
		ms, ok := Parse(in)
		if !ok {
			t.Fatal(in)
		}
		if got := PgTimestamp(ms); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestLocalTimeWithoutOffset(t *testing.T) {
	ms, ok := Parse("2026-11-21T10:00:00")
	want := time.Date(2026, 11, 21, 10, 0, 0, 0, time.Local).UnixMilli()
	if !ok || ms != want {
		t.Errorf("got %d %v, want %d", ms, ok, want)
	}
}
