package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

func init() {
	register("probe", "check a board: probe <apply-or-careers-url> | probe <ats> <token>", runProbe)
	register("manual-capture", "turn saved careers pages into data/manual/<ats>.json", runManualCapture)
}

func runProbe(args []string) error {
	flags := flag.NewFlagSet("probe", flag.ContinueOnError)
	full := flags.Bool("fetch", false, "read the whole board (as ingest does) and print its jobs as JSON")
	timeout := flags.Duration("timeout", 2*time.Minute, "give up after this long")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: openintern probe [-fetch] <apply-or-careers-url>\n       openintern probe [-fetch] <ats> <board_token>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	var atsName, boardSlug string
	switch flags.NArg() {
	case 1:
		parsed, ok := ats.ParseATSFromApplyURL(flags.Arg(0))
		if !ok {
			return fmt.Errorf("no supported ATS board in %s", flags.Arg(0))
		}
		atsName, boardSlug = parsed.ATS, parsed.BoardToken
		fmt.Printf("%s board %q (%s)\n", atsName, boardSlug, parsed.CareersURL)
	case 2:
		atsName, boardSlug = flags.Arg(0), flags.Arg(1)
	default:
		flags.Usage()
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if *full {
		jobs, err := ats.Fetch(ctx, atsName, boardSlug)
		if err != nil {
			return err
		}
		out, err := json.MarshalIndent(jobsJSON(jobs), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	if url, err := ats.ProbeURL(atsName, boardSlug); err == nil {
		fmt.Println(url)
	}
	r := ats.ProbeATSBoard(ctx, atsName, boardSlug)
	line := fmt.Sprintf("ok=%v status=%d", r.OK, r.Status)
	if r.Count != nil {
		line += fmt.Sprintf(" count=%d", *r.Count)
	}
	if r.Error != "" {
		line += " error=" + r.Error
	}
	fmt.Println(line)
	if !r.OK {
		return fmt.Errorf("%s %s: probe failed", atsName, boardSlug)
	}
	return nil
}

type jobJSON struct {
	ExternalID  string   `json:"externalId"`
	Title       string   `json:"title"`
	Locations   []string `json:"locations"`
	ApplyURL    string   `json:"applyUrl"`
	Excerpt     *string  `json:"excerpt"`
	PostedAt    *string  `json:"postedAt"`
	Description string   `json:"description"`
}

func jobsJSON(jobs []model.NormalizedJob) []jobJSON {
	out := make([]jobJSON, len(jobs))
	for i, j := range jobs {
		out[i] = jobJSON{j.ExternalID, j.Title, j.Locations, j.ApplyURL, j.Excerpt, j.PostedAt, j.Description}
	}
	return out
}

// runManualCapture is manual-capture.ts:
//
//	openintern manual-capture tesla ~/Downloads/state.json
//	openintern manual-capture citadel ~/Downloads/page1.html ~/Downloads/page2.html
//
// It writes data/manual/<ats>.json with normalized fields only; the saved
// pages are never committed.
func runManualCapture(args []string) error {
	flags := flag.NewFlagSet("manual-capture", flag.ContinueOnError)
	dir := flags.String("dir", filepath.Join("data", "manual"), "directory the capture is written to")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: openintern manual-capture <tesla|citadel|citadel_securities> <saved file>...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 2 {
		flags.Usage()
		os.Exit(2)
	}
	source, files := flags.Arg(0), flags.Args()[1:]

	var jobs []model.NormalizedJob
	var sourceURL string
	switch source {
	case "tesla":
		raw, err := os.ReadFile(files[0])
		if err != nil {
			return err
		}
		state, err := jsjson.Parse(jsjson.DecodeUTF8(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", files[0], err)
		}
		jobs = ats.ParseTeslaCareersState(state)
		sourceURL = ats.TeslaStateURL
	case "citadel", "citadel_securities":
		seen := map[string]bool{}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			for _, job := range ats.ParseCitadelListingHTML(jsjson.DecodeUTF8(raw), ats.CitadelOrigins[source]) {
				if !seen[job.ExternalID] {
					seen[job.ExternalID] = true
					jobs = append(jobs, job)
				}
			}
		}
		sourceURL = ats.CitadelListingURL(source, "open-opportunities", 1)
	default:
		fmt.Fprintf(os.Stderr, "no manual capture support for %s\n", source)
		os.Exit(2)
	}
	if len(jobs) == 0 {
		return fmt.Errorf("parsed zero listings; check that the saved page is the listing page")
	}
	capture, err := ats.ToManualCapture(jobs, sourceURL, time.Now(), "")
	if err != nil {
		return err
	}
	name, err := ats.ManualFile(source)
	if err != nil {
		return err
	}
	out := filepath.Join(*dir, name)
	if err := os.WriteFile(out, capture.MarshalFile(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %d jobs to %s (captured_at %s)\n", len(capture.Jobs), out, capture.CapturedAt)
	return nil
}
