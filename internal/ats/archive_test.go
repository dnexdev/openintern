package ats

import "testing"

func TestStripArchivePrefix(t *testing.T) {
	cases := map[string]string{
		"https://web.archive.org/web/20261101000000/https://www.citadel.com/careers/details/x/": "https://www.citadel.com/careers/details/x/",
		"http://web.archive.org/web/1/https://a.example/":                                       "https://a.example/",
		"/careers/details/x/":               "/careers/details/x/",
		"https://web.archive.org/web/abc/x": "https://web.archive.org/web/abc/x",
		"https://web.archive.org/web/123":   "https://web.archive.org/web/123",
	}
	for in, want := range cases {
		if got := stripArchivePrefix(in); got != want {
			t.Errorf("stripArchivePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
