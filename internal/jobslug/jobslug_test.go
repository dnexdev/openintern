package jobslug

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSlugifyTitleCases(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"Software Engineer Intern", "software-engineer-intern"},
		{"  Full Stack / ML Intern! ", "full-stack-ml-intern"},
		{"Ｆｕｌｌｗｉｄｔｈ ２０２７", "fullwidth-2027"},
		{"日本語インターン", "日本語インターン"},
		{"🧑‍💻🚀", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := SlugifyTitle(tc.title); got != tc.want {
			t.Errorf("SlugifyTitle(%q)=%q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestJobSlugFunctions(t *testing.T) {
	id := "1a2b3c4d-0000-0000-0000-000000000000"
	if got := JobSlugSegment("Software Engineer Intern", id); got != "software-engineer-intern-1a2b3c4d" {
		t.Fatalf("segment=%q", got)
	}
	if got := JobSlugSegment("🚀", id); got != "job-1a2b3c4d" {
		t.Fatalf("empty-base segment=%q", got)
	}
	if got := JobPath("example", "Software Engineer Intern", id); got != "/jobs/example/software-engineer-intern-1a2b3c4d" {
		t.Fatalf("path=%q", got)
	}
	want := "abcdef12"
	if got := ParseIdPrefixFromJobSlug("x-ABCDEF12"); got == nil || *got != want {
		t.Fatalf("parse=%#v", got)
	}
}

func TestTypeScriptGoldens(t *testing.T) {
	file, err := os.Open("testdata/goldens.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	var golden struct {
		RealCount      int `json:"realCount"`
		SyntheticCount int `json:"syntheticCount"`
		Cases          []struct {
			Input struct {
				Key         string `json:"key"`
				ID          string `json:"id"`
				Title       string `json:"title"`
				CompanySlug string `json:"companySlug"`
			} `json:"input"`
			Output struct {
				Slugified string  `json:"slugified"`
				Segment   string  `json:"segment"`
				Path      string  `json:"path"`
				Parsed    *string `json:"parsed"`
			} `json:"output"`
		} `json:"cases"`
		Malformed []struct {
			Slug   string  `json:"slug"`
			Parsed *string `json:"parsed"`
		} `json:"malformed"`
	}
	if err := json.NewDecoder(compressed).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if golden.RealCount != 1434 || golden.SyntheticCount < 4_000 || len(golden.Cases) != golden.RealCount+golden.SyntheticCount {
		t.Fatalf("unexpected corpus counts: real=%d synthetic=%d total=%d", golden.RealCount, golden.SyntheticCount, len(golden.Cases))
	}
	for _, tc := range golden.Cases {
		t.Run(tc.Input.Key, func(t *testing.T) {
			if got := SlugifyTitle(tc.Input.Title); got != tc.Output.Slugified {
				t.Fatalf("slugified: want %q, got %q", tc.Output.Slugified, got)
			}
			if got := JobSlugSegment(tc.Input.Title, tc.Input.ID); got != tc.Output.Segment {
				t.Fatalf("segment: want %q, got %q", tc.Output.Segment, got)
			}
			if got := JobPath(tc.Input.CompanySlug, tc.Input.Title, tc.Input.ID); got != tc.Output.Path {
				t.Fatalf("path: want %q, got %q", tc.Output.Path, got)
			}
			if got := ParseIdPrefixFromJobSlug(tc.Output.Segment); !reflect.DeepEqual(got, tc.Output.Parsed) {
				t.Fatalf("parsed: want %#v, got %#v", tc.Output.Parsed, got)
			}
		})
	}
	for _, tc := range golden.Malformed {
		if got := ParseIdPrefixFromJobSlug(tc.Slug); !reflect.DeepEqual(got, tc.Parsed) {
			t.Errorf("parse %q: want %#v, got %#v", tc.Slug, tc.Parsed, got)
		}
	}
}
