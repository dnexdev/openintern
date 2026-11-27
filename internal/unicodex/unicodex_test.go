package unicodex

import "testing"

// Expected values were produced by Node 26 (Unicode 17.0):
//
//	node -e 'console.log(JSON.stringify(s.normalize("NFKC")))'
func TestNFKC(t *testing.T) {
	cases := map[string]string{
		"Ｓｏｆｔｗａｒｅ　Ｉｎｔｅｒｎ": "Software Intern",
		"ﬁnance ﬂow":    "finance flow",
		"Intern ½ time": "Intern 1⁄2 time",
		"Café":          "Café",
		"Ⅳ":             "IV",
		"①":             "1",
	}
	for in, want := range cases {
		if got := NFKC(in); got != want {
			t.Errorf("NFKC(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtendedPictographic(t *testing.T) {
	yes := []rune{'🔥', '🔒', '🛂', '🎓', '©', '®', '‼', '⌚', 0x1FAE8}
	no := []rune{'a', '1', ' ', '-', 'é', '中', 0x1F1FA /* regional indicator U */, 0xFE0F, 0x200D}
	for _, r := range yes {
		if !IsExtendedPictographic(r) {
			t.Errorf("IsExtendedPictographic(%U) = false, want true", r)
		}
	}
	for _, r := range no {
		if IsExtendedPictographic(r) {
			t.Errorf("IsExtendedPictographic(%U) = true, want false", r)
		}
	}
}
