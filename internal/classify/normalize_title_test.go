package classify

import (
	"regexp"
	"testing"
)

func TestNormalizeTitleReferenceCases(t *testing.T) {
	palantirBase := NormalizeTitle("Forward Deployed Software Engineer, Internship - Commercial")
	for _, title := range []string{
		"Forward Deployed Software Engineer, Internship - USG",
		"Forward Deployed Software Engineer, Internship - AUS Government",
		"Forward Deployed Software Engineer, Internship (Fall 2026)",
	} {
		if got := NormalizeTitle(title); got != palantirBase {
			t.Errorf("%q normalized to %q, want %q", title, got, palantirBase)
		}
	}
	if NormalizeTitle("Software Engineering Intern (Summer 2027)") != NormalizeTitle("Software Engineering Intern") {
		t.Error("season parenthetical did not collapse")
	}
	if NormalizeTitle("Software Engineer Intern - San Francisco, CA") != NormalizeTitle("Software Engineer Intern - New York, NY") {
		t.Error("city suffixes did not collapse")
	}

	campusBase := NormalizeTitle("Software Engineer Intern - IIT Madras")
	for _, title := range []string{
		"Software Engineer Intern - IIT Bombay",
		"Software Engineer Intern - BITS Pilani",
		"Software Engineer Intern, Campus",
	} {
		if got := NormalizeTitle(title); got != campusBase {
			t.Errorf("%q normalized to %q, want %q", title, got, campusBase)
		}
	}
	if NormalizeTitle("2027 - Software Engineering Intern - IIT Delhi") != NormalizeTitle("Software Engineering Intern - IIT Bombay") {
		t.Error("leading cohort/campus variants did not collapse")
	}

	unequal := [][2]string{
		{"Campus Quantitative Researcher, UG/MS (Intern)", "Campus Quantitative Researcher, PhD (Intern)"},
		{"Quantitative Research Intern (PhD) - Summer 2027", "Quantitative Research Intern (BS/MS) - Summer 2027"},
		{"Research Intern (BS/MS/PhD)", "Research Intern"},
		{"Campus AI Research Engineer (Intern)", "Campus AI Research Engineer - Deep Learning (Intern)"},
		{"Campus AI Research Engineer (Intern)", "Campus AI Research Engineer – Research Automation (Intern)"},
	}
	for _, pair := range unequal {
		if NormalizeTitle(pair[0]) == NormalizeTitle(pair[1]) {
			t.Errorf("distinct titles collapsed: %q / %q", pair[0], pair[1])
		}
	}

	equal := [][2]string{
		{"Quantitative Research Intern (PhD) - Summer 2027 - Chicago", "Quantitative Research Intern (PhD) - Summer 2027 - Amsterdam"},
		{"Machine Learning Research Intern - Summer 2027 - Chicago", "Machine Learning Research Intern - Summer 2027 - Amsterdam"},
	}
	for _, pair := range equal {
		if NormalizeTitle(pair[0]) != NormalizeTitle(pair[1]) {
			t.Errorf("location variants differ: %q / %q", pair[0], pair[1])
		}
	}
}

func TestJobFingerprintReferenceCases(t *testing.T) {
	fingerprint := JobFingerprint("co-1", "greenhouse", "123")
	if len(fingerprint) != 64 {
		t.Fatalf("length=%d", len(fingerprint))
	}
	if JobFingerprint("co-1", "greenhouse", "123") != fingerprint {
		t.Fatal("fingerprint is unstable")
	}
	if JobFingerprint("co-1", "greenhouse", "124") == fingerprint {
		t.Fatal("different external ids collide")
	}
}

func TestRoleFamilyIdReferenceCases(t *testing.T) {
	base := NormalizeTitle("Forward Deployed Software Engineer, Internship - Commercial")
	id := RoleFamilyId("palantir", base)
	if !regexp.MustCompile(`^palantir:[a-f0-9]{10}$`).MatchString(id) {
		t.Fatalf("bad role family id %q", id)
	}
	if id != RoleFamilyId("palantir", NormalizeTitle("Forward Deployed Software Engineer, Internship - USG")) {
		t.Fatal("equivalent title changed role family id")
	}
}
