package tools

import "testing"

func TestHostInMatchesHostNotSubstring(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://x.com/acme", true},
		{"https://www.linkedin.com/company/acme", true},
		{"https://fedex.com", false},
		{"https://a16z.com.evil.example/", false},
		{"not a url", false},
	}
	for _, c := range cases {
		if got := hostIn(c.url, "x.com", "linkedin.com", "a16z.com"); got != c.want {
			t.Errorf("hostIn(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
