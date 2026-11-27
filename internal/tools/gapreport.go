package tools

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

type listSource struct{ id, url string }

// gapSources are the community lists gap-report reads.
var gapSources = []listSource{
	{"speedyapply-usa", "https://raw.githubusercontent.com/speedyapply/2027-SWE-College-Jobs/main/README.md"},
	{"speedyapply-intl", "https://raw.githubusercontent.com/speedyapply/2027-SWE-College-Jobs/main/INTERN_INTL.md"},
	{"simplify-summer", "https://raw.githubusercontent.com/SimplifyJobs/Summer2026-Internships/dev/README.md"},
	{"simplify-offseason", "https://raw.githubusercontent.com/SimplifyJobs/Summer2026-Internships/dev/README-Off-Season.md"},
}

// atsSources are the lists with markdown Apply links we can parse for ATS
// tokens.
var atsSources = gapSources[2:]

const markdownAccept = "text/plain,text/markdown,*/*"

// GapReportUsage is the usage text (the script's header comment).
const GapReportUsage = `usage: openintern gap-report [--json] [--top=N]
       openintern gap-report --ats [--write] [--json]

Diffs the SpeedyApply / SimplifyJobs lists against data/companies.
  --json      print the report as JSON
  --top=N     how many missing companies to list (default 60)
  --ats       extract ATS boards from Apply links (alias --from-apply) and probe the missing ones
  --write     with --ats, append probe-OK missing boards to data/companies/tech-c.yaml
`

// GapReport is gap-report.ts. args are the script's arguments.
func GapReport(ctx context.Context, env Env, args []string) error {
	asJSON := hasArg(args, "--json")
	atsMode := hasArg(args, "--ats", "--from-apply")
	write := hasArg(args, "--write")
	topN := 60.0
	if n, ok := numberArg(args, "--top="); ok {
		topN = math.Max(1, orDefault(n, 60))
	}
	if atsMode {
		return runATSReport(ctx, env, asJSON, write)
	}
	return runNameReport(ctx, env, asJSON, topN)
}

type atsCandidate struct {
	name, slug, ats, boardToken, careersURL, applyURL string
	sources                                           []string
	registrySlug                                      string
}

func uniqueBoards(hits []sourcedHit) []*atsCandidate {
	var out []*atsCandidate
	byKey := map[string]*atsCandidate{}
	for _, h := range hits {
		key := ats.TokenKey(h.hit.ATS, h.hit.BoardToken)
		if prev, ok := byKey[key]; ok {
			if !slices.Contains(prev.sources, h.source) {
				prev.sources = append(prev.sources, h.source)
			}
			continue
		}
		c := &atsCandidate{
			name: h.hit.CompanyName, slug: Slugify(h.hit.CompanyName), ats: h.hit.ATS,
			boardToken: h.hit.BoardToken, careersURL: h.hit.CareersURL,
			sources: []string{h.source}, applyURL: h.hit.ApplyURL,
		}
		byKey[key] = c
		out = append(out, c)
	}
	return out
}

type sourcedHit struct {
	hit    ats.ATSHit
	source string
}

func disambiguateSlug(base string, used map[string]bool, atsName, token string) string {
	slug := base
	if slug == "" {
		slug = Slugify(token)
	}
	if !used[slug] {
		return slug
	}
	withToken := collapseHyphens(base + "-" + Slugify(token))
	if !used[withToken] {
		return withToken
	}
	withATS := base + "-" + atsName
	if !used[withATS] {
		return withATS
	}
	i := 2
	for used[fmt.Sprintf("%s-%d", base, i)] {
		i++
	}
	return fmt.Sprintf("%s-%d", base, i)
}

var yamlSpecial = regexp.MustCompile("[:#{}\\[\\],&*?|>!%@`]")

func runATSReport(ctx context.Context, env Env, asJSON, write bool) error {
	reg, err := env.loadRegistryIndex()
	if err != nil {
		return err
	}
	var allHits []sourcedHit
	applyLinkCount, unsupportedApplyCount := 0, 0
	var hostOrder []string
	hostCount := map[string]int{}
	for _, src := range atsSources {
		md, err := fetchText(ctx, src.url, markdownAccept)
		if err != nil {
			fmt.Fprintf(env.Stderr, "warn: failed %s: %s\n", src.id, err)
			continue
		}
		ex := ats.ExtractATSBoardsFromMarkdown(md)
		applyLinkCount += ex.ApplyLinkCount
		unsupportedApplyCount += ex.UnsupportedApplyCount
		for _, h := range ex.UnsupportedHosts {
			if _, seen := hostCount[h.Host]; !seen {
				hostOrder = append(hostOrder, h.Host)
			}
			hostCount[h.Host] += h.Count
		}
		for _, h := range ex.Hits {
			allHits = append(allHits, sourcedHit{h, src.id})
		}
	}

	boards := uniqueBoards(allHits)
	var covered, inactive, missing []*atsCandidate
	for _, c := range boards {
		if byTok, ok := reg.byToken[ats.TokenKey(c.ats, c.boardToken)]; ok {
			if byTok.Active {
				covered = append(covered, c)
			} else {
				cc := *c
				cc.registrySlug = byTok.Slug
				inactive = append(inactive, &cc)
			}
			continue
		}
		// A known company name under a different token is still a missing
		// board: only a token match counts as covered.
		missing = append(missing, c)
	}

	sorted := slices.Clone(missing)
	slices.SortStableFunc(sorted, func(a, b *atsCandidate) int { return localeCompare(a.name, b.name) })
	type probed struct {
		c  *atsCandidate
		ok bool
	}
	var okList, probeFail []*atsCandidate
	inChunks(ctx, sorted, 8, func(ctx context.Context, c *atsCandidate) probed {
		return probed{c, probeOK(ctx, c.ats, c.boardToken)}
	}, func(rs []probed) {
		for _, r := range rs {
			if r.ok {
				okList = append(okList, r.c)
			} else {
				probeFail = append(probeFail, r.c)
			}
		}
	})

	var written []registry.CompanyYAML
	if write && len(okList) > 0 {
		if written, err = appendTechC(env, reg, okList); err != nil {
			return err
		}
	}

	var atsOrder []string
	byATS := map[string]int{}
	for _, c := range boards {
		if _, ok := byATS[c.ats]; !ok {
			atsOrder = append(atsOrder, c.ats)
		}
		byATS[c.ats]++
	}
	byATSObj := jsjson.Object{}
	for _, a := range atsOrder {
		byATSObj = append(byATSObj, jsjson.Member{Key: a, Value: byATS[a]})
	}
	slices.SortStableFunc(hostOrder, func(a, b string) int { return hostCount[b] - hostCount[a] })
	hosts := jsjson.Object{}
	for _, h := range hostOrder {
		hosts = append(hosts, jsjson.Member{Key: h, Value: hostCount[h]})
	}

	coveredJ := []any{}
	for _, c := range covered {
		coveredJ = append(coveredJ, obj("name", c.name, "ats", c.ats, "board_token", c.boardToken))
	}
	inactiveJ := []any{}
	for _, c := range inactive {
		inactiveJ = append(inactiveJ, obj("name", c.name, "ats", c.ats, "board_token", c.boardToken, "registrySlug", c.registrySlug))
	}
	okJ := []any{}
	for _, c := range okList {
		okJ = append(okJ, obj("name", c.name, "slug", c.slug, "ats", c.ats, "board_token", c.boardToken, "careers_url", c.careersURL, "sources", stringsAny(c.sources)))
	}
	failJ := []any{}
	for _, c := range probeFail {
		failJ = append(failJ, obj("name", c.name, "ats", c.ats, "board_token", c.boardToken, "reason", "probe failed"))
	}
	writtenJ := []any{}
	for _, c := range written {
		writtenJ = append(writtenJ, obj("name", c.Name, "slug", c.Slug, "ats", c.ATS, "board_token", c.BoardToken))
	}
	report := obj(
		"generatedAt", isoTime(env.now()),
		"mode", "ats",
		"registrySize", len(reg.bySlug),
		"applyLinkCount", applyLinkCount,
		"unsupportedApplyCount", unsupportedApplyCount,
		"unsupportedHosts", hosts,
		"uniqueSupportedTokens", len(boards),
		"byAts", byATSObj,
		"coveredCount", len(covered),
		"inactiveCount", len(inactive),
		"missingCount", len(missing),
		"probeOkCount", len(okList),
		"probeFailCount", len(probeFail),
		"writtenCount", len(written),
		"covered", coveredJ,
		"inactive", inactiveJ,
		"missingProbeOk", okJ,
		"missingProbeFail", failJ,
		"written", writtenJ,
		"tokenElsewhere", []any{},
	)
	if asJSON {
		fmt.Fprintf(env.Stdout, "%s\n", jsjson.MarshalIndent(report, "  "))
		return nil
	}

	w := env.Stdout
	fmt.Fprint(w, "## SimplifyJobs ATS volume report\n\n")
	fmt.Fprintf(w, "Registry: %d companies\n", len(reg.bySlug))
	fmt.Fprintf(w, "Apply links: %d\n", applyLinkCount)
	fmt.Fprintf(w, "  supported (parsed board): %d\n", applyLinkCount-unsupportedApplyCount)
	fmt.Fprintf(w, "  unsupported hosts: %d\n", unsupportedApplyCount)
	fmt.Fprintf(w, "Unique supported board tokens: %d\n", len(boards))
	fmt.Fprintf(w, "  by ATS: %s\n", jsjson.Marshal(byATSObj))
	fmt.Fprintf(w, "Already covered (active): %d\n", len(covered))
	fmt.Fprintf(w, "In registry but inactive: %d\n", len(inactive))
	fmt.Fprintf(w, "Missing from registry: %d\n", len(missing))
	fmt.Fprintf(w, "  probe OK (addable): %d\n", len(okList))
	fmt.Fprintf(w, "  probe failed: %d\n", len(probeFail))
	if write {
		fmt.Fprintf(w, "Written to tech-c.yaml: %d\n", len(written))
	}
	fmt.Fprint(w, "\n### Top unsupported hosts\n\n")
	for _, h := range hostOrder[:min(15, len(hostOrder))] {
		fmt.Fprintf(w, "- %s: %d\n", h, hostCount[h])
	}
	fmt.Fprint(w, "\n### Missing (probe OK)\n\n")
	for _, m := range okList {
		fmt.Fprintf(w, "- %s  ats=%s  token=%s  slug=%s  [%s]\n", m.name, m.ats, m.boardToken, m.slug, strings.Join(m.sources, ", "))
	}
	if len(probeFail) > 0 {
		fmt.Fprint(w, "\n### Missing (probe failed)\n\n")
		for _, m := range probeFail {
			fmt.Fprintf(w, "- %s  ats=%s  token=%s\n", m.name, m.ats, m.boardToken)
		}
	}
	if len(inactive) > 0 {
		fmt.Fprint(w, "\n### Inactive but listed (re-activate?)\n\n")
		for _, m := range inactive {
			fmt.Fprintf(w, "- %s  (%s)  ats=%s  token=%s\n", m.name, m.registrySlug, m.ats, m.boardToken)
		}
	}
	if len(written) > 0 {
		fmt.Fprint(w, "\n### Written\n\n")
		for _, c := range written {
			fmt.Fprintf(w, "- %s (%s) %s/%s\n", c.Name, c.Slug, c.ATS, c.BoardToken)
		}
	}
	fmt.Fprint(w, "\nNext: openintern validate-companies && openintern validate-tokens data/companies/tech-c.yaml\n")
	fmt.Fprint(w, "Then: openintern sync-companies && ingest (when ready).\n")
	fmt.Fprint(w, "YC/a16z deferred until after reviewing this volume.\n\n")
	return nil
}

// appendTechC appends the probe-OK boards to data/companies/tech-c.yaml,
// checking the result parses before writing it.
func appendTechC(env Env, reg *registryIndex, probeOK []*atsCandidate) ([]registry.CompanyYAML, error) {
	used := map[string]bool{}
	for s := range reg.bySlug {
		used[s] = true
	}
	var additions []registry.CompanyYAML
	for _, c := range probeOK {
		if _, ok := reg.byToken[ats.TokenKey(c.ats, c.boardToken)]; ok {
			continue
		}
		slug := disambiguateSlug(c.slug, used, c.ats, c.boardToken)
		used[slug] = true
		careers := c.careersURL
		additions = append(additions, registry.CompanyYAML{Name: c.name, Slug: slug, ATS: c.ats, BoardToken: c.boardToken, CareersURL: &careers, Active: true})
	}
	if len(additions) == 0 {
		return nil, nil
	}
	var block []string
	for _, e := range additions {
		name := e.Name
		if yamlSpecial.MatchString(name) || strings.Contains(name, "'") {
			name = jsonQuote(name)
		}
		token := e.BoardToken
		if yamlSpecial.MatchString(token) || strings.Contains(token, ".") {
			token = jsonQuote(token)
		}
		block = append(block, strings.Join([]string{
			"  - name: " + name,
			"    slug: " + e.Slug,
			"    ats: " + e.ATS,
			"    board_token: " + token,
			"    careers_url: " + *e.CareersURL,
		}, "\n"))
	}
	path := filepath.Join(env.companiesDir(), "tech-c.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := string(src)
	if !strings.HasSuffix(raw, "\n") {
		raw += "\n"
	}
	raw += "\n# Added from SimplifyJobs Apply-URL extraction (" + isoDate(env.now()) + ")\n"
	raw += strings.Join(block, "\n") + "\n"
	if _, _, err := registry.ParseCompaniesFile("data/companies/tech-c.yaml", []byte(raw)); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		return nil, err
	}
	return additions, nil
}

type nameHit struct {
	name, slug string
	sources    []string
}

func runNameReport(ctx context.Context, env Env, asJSON bool, topN float64) error {
	reg, err := env.loadRegistryIndex()
	if err != nil {
		return err
	}
	var missingOrder, inactiveOrder []string
	seenMissing := map[string]*nameHit{}
	seenInactive := map[string]*nameHit{}
	addSource := func(h *nameHit, id string) {
		if !slices.Contains(h.sources, id) {
			h.sources = append(h.sources, id)
		}
	}
	total := 0
	for _, src := range gapSources {
		md, err := fetchText(ctx, src.url, markdownAccept)
		if err != nil {
			fmt.Fprintf(env.Stderr, "warn: failed %s: %s\n", src.id, err)
			continue
		}
		names := ExtractCompaniesFromMarkdown(md)
		total += len(names)
		for _, name := range names {
			norm := normalizeName(name)
			if hit, ok := reg.byNorm[norm]; ok {
				if !hit.Active {
					if prev, ok := seenInactive[hit.Slug]; ok {
						addSource(prev, src.id)
					} else {
						seenInactive[hit.Slug] = &nameHit{name: hit.Name, slug: hit.Slug, sources: []string{src.id}}
						inactiveOrder = append(inactiveOrder, hit.Slug)
					}
				}
				continue
			}
			if prev, ok := seenMissing[norm]; ok {
				addSource(prev, src.id)
			} else {
				seenMissing[norm] = &nameHit{name: name, sources: []string{src.id}}
				missingOrder = append(missingOrder, norm)
			}
		}
	}
	collect := func(order []string, seen map[string]*nameHit) []*nameHit {
		out := make([]*nameHit, 0, len(order))
		for _, k := range order {
			h := seen[k]
			slices.SortFunc(h.sources, jsjson.CompareCodeUnits)
			out = append(out, h)
		}
		slices.SortStableFunc(out, func(a, b *nameHit) int { return localeCompare(a.name, b.name) })
		return out
	}
	missing := collect(missingOrder, seenMissing)
	inactive := collect(inactiveOrder, seenInactive)
	for _, m := range missing {
		m.slug = Slugify(m.name)
	}
	topMissing := sliceTo(missing, topN)
	topInactive := sliceTo(inactive, math.Min(topN, 100))

	if asJSON {
		missingJ := []any{}
		for _, m := range topMissing {
			missingJ = append(missingJ, obj("name", m.name, "slug_guess", m.slug, "sources", stringsAny(m.sources)))
		}
		inactiveJ := []any{}
		for _, m := range topInactive {
			inactiveJ = append(inactiveJ, obj("name", m.name, "slug", m.slug, "sources", stringsAny(m.sources)))
		}
		report := obj(
			"generatedAt", isoTime(env.now()),
			"registrySize", len(reg.bySlug),
			"externalCompanyMentions", total,
			"missingCount", len(missing),
			"inactiveButListedCount", len(inactive),
			"top", jsonNumber(topN),
			"missing", missingJ,
			"inactiveButListed", inactiveJ,
		)
		fmt.Fprintf(env.Stdout, "%s\n", jsjson.MarshalIndent(report, "  "))
		return nil
	}

	w := env.Stdout
	fmt.Fprintf(w, "Registry: %d companies\n", len(reg.bySlug))
	fmt.Fprintf(w, "External mentions parsed: %d\n", total)
	fmt.Fprintf(w, "Missing from registry: %d\n", len(missing))
	fmt.Fprintf(w, "In registry but inactive (listed externally): %d\n", len(inactive))
	fmt.Fprint(w, "\n## Top missing (add YAML + board token)\n\n")
	for _, m := range topMissing {
		fmt.Fprintf(w, "- %s  (slug: %s)  [%s]\n", m.name, m.slug, strings.Join(m.sources, ", "))
		fmt.Fprint(w, "    # suggested starter:\n")
		fmt.Fprintf(w, "    # - name: %s\n", m.name)
		fmt.Fprintf(w, "    #   slug: %s\n", m.slug)
		fmt.Fprint(w, "    #   ats: greenhouse  # verify with openintern validate-tokens\n")
		fmt.Fprint(w, "    #   board_token: CHANGE_ME\n")
		fmt.Fprint(w, "    #   website_url: https://www.example.com\n")
	}
	fmt.Fprint(w, "\n## Inactive but still listed externally (recover token?)\n\n")
	for _, m := range topInactive[:min(40, len(topInactive))] {
		fmt.Fprintf(w, "- %s  (%s)  [%s]\n", m.name, m.slug, strings.Join(m.sources, ", "))
	}
	return nil
}

// obj builds an ordered JSON object from key, value pairs.
func obj(kv ...any) jsjson.Object {
	o := make(jsjson.Object, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		o = append(o, jsjson.Member{Key: kv[i].(string), Value: kv[i+1]})
	}
	return o
}

func stringsAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
