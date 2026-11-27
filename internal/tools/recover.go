package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state/jsjson"
	"github.com/dnexdev/openintern/internal/yamlite"
)

// prioritySlugs are high-value inactive or often-wrong brands to try first.
var prioritySlugs = []string{
	"bloomberg", "sig", "xai", "shopify", "nvidia", "amd", "ibm", "wealthsimple",
	"google", "microsoft", "deepmind", "deshaw", "ramp", "stripe", "imc", "palantir",
	"netflix", "adobe", "atlassian", "doordash", "salesforce", "intuit", "zoom", "slack",
	"uber", "lyft", "airbnb", "dropbox", "twitch", "robinhood", "coinbase", "databricks",
	"snowflake", "mongodb", "elastic", "gitlab", "hashicorp", "cloudflare", "okta", "twilio",
	"hubspot", "asana", "notion", "figma", "canva", "docusign", "zendesk", "splunk",
	"crowdstrike", "palo-alto-networks", "two-sigma", "hrt", "citadel", "jane-street", "optiver", "drw",
	"akuna", "tesla", "spacex", "anduril", "rivian", "sony", "samsung", "intel",
	"qualcomm", "broadcom", "cisco", "oracle", "sap", "workday", "servicenow", "autodesk",
	"ea", "activision", "riot-games", "roblox", "unity", "epic-games",
}

var (
	reTokenChars = regexp.MustCompile(`(?i)^[a-z0-9-]+$`)
)

func titleCaseToken(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		parts[i] = firstRuneUpper(p)
	}
	return strings.Join(parts, "")
}

// splitJSSpace is `s.split(/\s+/)`.
func splitJSSpace(s string) []string {
	var out []string
	cur := strings.Builder{}
	inSpace := false
	for _, r := range s {
		if htmlx.IsJSSpace(r) {
			if !inSpace {
				out = append(out, cur.String())
				cur.Reset()
			}
			inSpace = true
			continue
		}
		inSpace = false
		cur.WriteRune(r)
	}
	return append(out, cur.String())
}

// recoverTokenCandidates are the spellings tried for a company on one ATS.
func recoverTokenCandidates(slug, name, current, atsName string) []string {
	compact := asciiAlnum(jsLower(name))
	nospace := strings.ReplaceAll(slug, "-", "")
	first := strings.Split(slug, "-")[0]
	title := titleCaseToken(slug)
	set := newOrderedSet(current, slug, nospace, compact, first, title, nospace+"careers", first+"careers")
	if atsName == "smartrecruiters" {
		set.add(title)
		set.add(splitJSSpace(name)[0])
		for _, part := range splitJSSpace(name) {
			if len16(part) >= 2 {
				set.add(part)
			}
		}
	}
	var out []string
	for _, t := range set.list {
		if len16(t) < 2 {
			continue
		}
		if (atsName == "recruitee" || atsName == "bamboohr") && !reTokenChars.MatchString(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

type recoverProbe struct {
	ok    bool
	count float64
}

// recoverProbeBoard is recover-tokens' own probe: one GET of the probe URL;
// any non-2xx answer (429 included) or a body that is not JSON is a miss.
func recoverProbeBoard(ctx context.Context, atsName, token string) recoverProbe {
	url, err := ats.ProbeURL(atsName, token)
	if err != nil {
		return recoverProbe{}
	}
	res, err := fetch.Request(ctx, url, fetch.Options{Attempts: 1, Headers: map[string]string{"Accept": "application/json"}})
	if err != nil {
		return recoverProbe{}
	}
	v, err := jsjson.Parse(decodeBody(res.Body))
	if err != nil {
		return recoverProbe{}
	}
	return recoverProbe{ok: true, count: ats.CountJobsFromProbeBody(atsName, v)}
}

type recovery struct {
	ats, token string
	count      float64
}

func findRecovery(ctx context.Context, c registry.CompanyYAML) *recovery {
	order := []string{}
	if slices.Contains(ats.PublicATS, c.ATS) {
		order = append(order, c.ATS)
	}
	for _, a := range ats.PublicATS {
		if a != c.ATS {
			order = append(order, a)
		}
	}
	for _, a := range order {
		for _, token := range recoverTokenCandidates(c.Slug, c.Name, c.BoardToken, a) {
			r := recoverProbeBoard(ctx, a, token)
			if !r.ok {
				continue
			}
			if c.Active && a == c.ATS && token == c.BoardToken {
				continue
			}
			return &recovery{ats: a, token: token, count: r.count}
		}
	}
	return nil
}

type suggestion struct {
	company registry.CompanyYAML
	to      recovery
}

// RecoverTokens is recover-tokens.ts.
func RecoverTokens(ctx context.Context, env Env, args []string) error {
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--" })
	limit := 40.0
	if n, ok := numberArg(args, "--limit="); ok {
		limit = n
	}
	var only map[string]bool
	if v, ok := argValue(args, "--slug="); ok {
		only = map[string]bool{}
		for _, s := range strings.Split(v, ",") {
			only[htmlx.TrimJS(s)] = true
		}
	}
	write := hasArg(args, "--write")

	companies, err := env.loadCompanyRows()
	if err != nil {
		return err
	}
	var targets []registry.CompanyYAML
	if only != nil {
		for _, c := range companies {
			if only[c.Slug] {
				targets = append(targets, c)
			}
		}
	} else {
		for _, s := range prioritySlugs {
			for _, c := range companies {
				if c.Slug == s {
					targets = append(targets, c)
					break
				}
			}
		}
		for _, c := range companies {
			if !c.Active && !slices.Contains(prioritySlugs, c.Slug) {
				targets = append(targets, c)
			}
		}
		targets = sliceTo(targets, limit)
	}

	var suggestions []suggestion
	for _, c := range targets {
		fmt.Fprintf(env.Stderr, "probing %s...\n", c.Slug)
		found := findRecovery(ctx, c)
		if found == nil || found.ats == c.ATS && found.token == c.BoardToken && c.Active {
			continue
		}
		suggestions = append(suggestions, suggestion{c, *found})
		fmt.Fprintf(env.Stdout, "RECOVER %s: %s/%s → %s/%s (%s jobs)\n", c.Slug, c.ATS, c.BoardToken, found.ats, found.token, jsString(found.count))
	}
	var useful []suggestion
	for _, s := range suggestions {
		if s.to.count > 0 {
			useful = append(useful, s)
		}
	}
	w := env.Stdout
	fmt.Fprintf(w, "\n# %d recovery suggestion(s) with jobs (of %d probes)\n\n", len(useful), len(suggestions))
	for _, s := range useful {
		c := s.company
		fmt.Fprintf(w, "# %s (%s) — was %s/%s\n", c.Name, c.Slug, c.ATS, c.BoardToken)
		fmt.Fprintf(w, "  - name: %s\n", c.Name)
		fmt.Fprintf(w, "    slug: %s\n", c.Slug)
		fmt.Fprintf(w, "    ats: %s\n", s.to.ats)
		fmt.Fprintf(w, "    board_token: %s\n", s.to.token)
		fmt.Fprint(w, "    active: true\n")
		if c.WebsiteURL != nil && *c.WebsiteURL != "" {
			fmt.Fprintf(w, "    website_url: %s\n", *c.WebsiteURL)
		} else {
			fmt.Fprintf(w, "    website_url: https://www.%s.com\n", strings.ReplaceAll(c.Slug, "-", ""))
		}
		fmt.Fprintln(w)
	}
	if write && len(useful) > 0 {
		for _, s := range useful {
			if err := writeRecovery(env, s); err != nil {
				return err
			}
			fmt.Fprintf(w, "wrote %s → %s\n", s.company.Slug, s.company.File)
		}
	}
	return nil
}

// writeRecovery points every entry with the suggestion's slug in its file
// at the recovered board and marks it active. It edits those lines in place
// (ats, board_token, active), so the file's comments, order and quoting
// survive; the script re-serialized the whole file instead.
func writeRecovery(env Env, s suggestion) error {
	path := filepath.Join(env.companiesDir(), s.company.File)
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, changed, err := setCompanyBoard(path, src, s.company.Slug, s.to.ats, s.to.token)
	if err != nil || !changed {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// setCompanyBoard rewrites ats, board_token and active of every companies
// entry whose slug is slug. The result is parsed again and must validate.
func setCompanyBoard(file string, src []byte, slug, atsName, token string) ([]byte, bool, error) {
	root, err := yamlite.Parse(file, src)
	if err != nil {
		return nil, false, err
	}
	list := root.Get("companies")
	if list == nil || list.Kind != yamlite.Seq {
		return src, false, nil
	}
	lines := strings.SplitAfter(string(src), "\n")
	type edit struct {
		line   int // 1-based line to replace, or insert after when insert is set
		text   string
		insert bool
	}
	var edits []edit
	for _, it := range list.Items {
		if it.Kind != yamlite.Map {
			continue
		}
		if n := it.Get("slug"); n == nil || n.Kind != yamlite.String || n.Str != slug {
			continue
		}
		var tokenLine, lastLine int
		hasActive := false
		for _, p := range it.Pairs {
			lastLine = max(lastLine, p.Line)
			switch p.Key {
			case "ats":
				edits = append(edits, edit{line: p.Line, text: replaceValue(lines[p.Line-1], "ats", atsName)})
			case "board_token":
				tokenLine = p.Line
				edits = append(edits, edit{line: p.Line, text: replaceValue(lines[p.Line-1], "board_token", yamlScalar(token))})
			case "active":
				hasActive = true
				edits = append(edits, edit{line: p.Line, text: replaceValue(lines[p.Line-1], "active", "true")})
			}
		}
		if !hasActive {
			at := tokenLine
			if at == 0 {
				at = lastLine
			}
			edits = append(edits, edit{line: at, text: keyIndent(lines[at-1]) + "active: true\n", insert: true})
		}
	}
	if len(edits) == 0 {
		return src, false, nil
	}
	slices.SortStableFunc(edits, func(a, b edit) int { return b.line - a.line })
	for _, e := range edits {
		if e.insert {
			prev := lines[e.line-1]
			if !strings.HasSuffix(prev, "\n") {
				lines[e.line-1] = prev + "\n"
			}
			lines = slices.Insert(lines, e.line, e.text)
		} else {
			lines[e.line-1] = e.text
		}
	}
	out := []byte(strings.Join(lines, ""))
	list2, _, err := registry.ParseCompaniesFile(file, out)
	if err != nil {
		return nil, false, fmt.Errorf("rewriting %s for %s: %w", file, slug, err)
	}
	for _, c := range list2 {
		if c.Slug == slug && (c.ATS != atsName || c.BoardToken != token || !c.Active) {
			return nil, false, fmt.Errorf("rewriting %s for %s: entry did not take the new board", file, slug)
		}
	}
	return out, true, nil
}

// keyIndent is the indentation of a mapping key on line ("    " for
// "    ats: x", and for "  - ats: x" the column after "- ").
func keyIndent(line string) string {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '-') {
		n++
	}
	return strings.Repeat(" ", n)
}

// replaceValue rewrites `<indent>key: value  # comment` keeping the
// indentation and a trailing comment.
func replaceValue(line, key, value string) string {
	nl := ""
	if strings.HasSuffix(line, "\n") {
		nl = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	i := strings.Index(line, key+":")
	if i < 0 {
		return line + nl
	}
	comment := ""
	if j := commentStart(line[i+len(key)+1:]); j >= 0 {
		comment = " " + line[i+len(key)+1+j:]
	}
	return line[:i] + key + ": " + value + comment + nl
}

// commentStart finds " #" outside quotes in a YAML value.
func commentStart(v string) int {
	var quote byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' {
				i++
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' && i > 0 && (v[i-1] == ' ' || v[i-1] == '\t'):
			return i
		}
	}
	return -1
}

// yamlScalar writes a token plain when the registry parser reads it back
// as the same string, quoted otherwise.
func yamlScalar(s string) string {
	n, err := yamlite.Parse("token", []byte("v: "+s+"\n"))
	if err == nil {
		if v := n.Get("v"); v != nil && v.Kind == yamlite.String && v.Str == s && !yamlSpecial.MatchString(s) {
			return s
		}
	}
	return jsonQuote(s)
}
