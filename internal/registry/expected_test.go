package registry

import "testing"

func TestExpectedDelta(t *testing.T) {
	d, err := ParseExpectedDelta([]byte(`{"approvals":[
		{"scope":"total","until":"2026-12-31","reason":"x"},
		{"scope":"ats:lever","until":"2026-12-31","reason":"  "},
		{"scope":"ats:ashby","until":20261231,"reason":"x"},
		{"scope":"company:a","until":"2026-11-01","reason":"x"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		scope, today string
		want         bool
	}{
		{"total", "2026-12-31", true},
		{"total", "2027-01-01", false},
		{"ats:lever", "2026-11-01", false}, // blank reason
		{"ats:ashby", "2026-11-01", false}, // until is not a string
		{"company:a", "2026-11-01", true},
		{"company:a", "2026-11-02", false},
		{"company:b", "2026-11-01", false},
	} {
		if got := d.Approved(c.scope, c.today); got != c.want {
			t.Errorf("%s on %s: got %v", c.scope, c.today, got)
		}
	}
	if _, err := ParseExpectedDelta([]byte(`{}`)); err == nil {
		t.Error("missing approvals list must fail")
	}
}
