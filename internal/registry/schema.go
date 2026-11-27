package registry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/registry/weburl"
	"github.com/dnexdev/openintern/internal/yamlite"
)

// CompanyYAML is one entry of data/companies/*.yaml (TS: CompanyYaml).
type CompanyYAML struct {
	Name       string
	Slug       string
	ATS        string
	BoardToken string
	// ExtraBoardTokens are more boards on the same ATS for this company;
	// postings merge by id. Nil when the key is absent.
	ExtraBoardTokens []string
	CareersURL       *string
	WebsiteURL       *string
	Active           bool // default true

	File string // file name inside data/companies, e.g. "tech-a.yaml"
	Line int    // line of the entry's first key
}

// Issue is one schema violation, worded like the TypeScript (zod) message.
type Issue struct {
	Path    string // "companies[3].slug"
	Message string
	Line    int
}

// SchemaError lists every issue in one file.
type SchemaError struct {
	File   string
	Issues []Issue
}

func (e *SchemaError) Error() string {
	var b strings.Builder
	for i, is := range e.Issues {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s:%d: %s: %s", e.File, is.Line, is.Path, is.Message)
	}
	return b.String()
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

const slugMessage = "slug must be lowercase alphanumeric with hyphens"

var atsMessage = func() string {
	quoted := make([]string, len(model.AllATS))
	for i, a := range model.AllATS {
		quoted[i] = strconv.Quote(a)
	}
	return "Invalid option: expected one of " + strings.Join(quoted, "|")
}()

func isATS(s string) bool {
	for _, a := range model.AllATS {
		if a == s {
			return true
		}
	}
	return false
}

// checker collects issues the way zod reports them: every field of the
// schema in declaration order, every problem of a field.
type checker struct {
	issues []Issue
}

func (c *checker) add(path string, line int, msg string) {
	c.issues = append(c.issues, Issue{Path: path, Message: msg, Line: line})
}

func (c *checker) typeIssue(path string, line int, want string, got *yamlite.Node) {
	c.add(path, line, fmt.Sprintf("Invalid input: expected %s, received %s", want, got.TypeName()))
}

// str checks a required or optional string; ok is false when it is absent
// or not a string.
func (c *checker) str(path string, parentLine int, n *yamlite.Node, optional bool) (string, bool) {
	if n == nil && optional {
		return "", false
	}
	if n == nil || n.Kind != yamlite.String {
		line := parentLine
		if n != nil {
			line = n.Line
		}
		c.typeIssue(path, line, "string", n)
		return "", false
	}
	return n.Str, true
}

func (c *checker) nonEmpty(path string, line int, s string) {
	if s == "" {
		c.add(path, line, "Too small: expected string to have >=1 characters")
	}
}

func (c *checker) slug(path string, line int, s string) {
	c.nonEmpty(path, line, s)
	if !slugRe.MatchString(s) {
		c.add(path, line, slugMessage)
	}
}

// url is z.string().url(): the trimmed value must parse with `new URL`, and
// the stored value is trimmed with tabs and newlines removed.
func (c *checker) url(path string, parentLine int, n *yamlite.Node) *string {
	s, ok := c.str(path, parentLine, n, true)
	if !ok {
		return nil
	}
	trimmed := jsTrim(s)
	if !weburl.CanParse(trimmed) {
		c.add(path, n.Line, "Invalid URL")
		return nil
	}
	v := strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(trimmed)
	return &v
}

// knownCompanyKeys are the keys of companyYamlSchema. Others are ignored, as
// zod's object schema strips them; Unknown reports them for a warning.
var knownCompanyKeys = map[string]bool{
	"name": true, "slug": true, "ats": true, "board_token": true, "extra_board_tokens": true,
	"careers_url": true, "website_url": true, "active": true,
}

func parseCompany(c *checker, path string, n *yamlite.Node) CompanyYAML {
	out := CompanyYAML{Line: n.Line, Active: true}
	if n.Kind != yamlite.Map {
		c.typeIssue(path, n.Line, "object", n)
		return out
	}
	if s, ok := c.str(path+".name", n.Line, n.Get("name"), false); ok {
		c.nonEmpty(path+".name", n.Get("name").Line, s)
		out.Name = s
	}
	if s, ok := c.str(path+".slug", n.Line, n.Get("slug"), false); ok {
		c.slug(path+".slug", n.Get("slug").Line, s)
		out.Slug = s
	}
	if v := n.Get("ats"); v == nil || v.Kind != yamlite.String || !isATS(v.Str) {
		line := n.Line
		if v != nil {
			line = v.Line
		}
		c.add(path+".ats", line, atsMessage)
	} else {
		out.ATS = v.Str
	}
	if s, ok := c.str(path+".board_token", n.Line, n.Get("board_token"), false); ok {
		c.nonEmpty(path+".board_token", n.Get("board_token").Line, s)
		out.BoardToken = s
	}
	if v := n.Get("extra_board_tokens"); v != nil {
		if v.Kind != yamlite.Seq {
			c.typeIssue(path+".extra_board_tokens", v.Line, "array", v)
		} else {
			out.ExtraBoardTokens = []string{}
			for i, it := range v.Items {
				p := fmt.Sprintf("%s.extra_board_tokens[%d]", path, i)
				if s, ok := c.str(p, it.Line, it, false); ok {
					c.nonEmpty(p, it.Line, s)
					out.ExtraBoardTokens = append(out.ExtraBoardTokens, s)
				}
			}
		}
	}
	out.CareersURL = c.url(path+".careers_url", n.Line, n.Get("careers_url"))
	out.WebsiteURL = c.url(path+".website_url", n.Line, n.Get("website_url"))
	if v := n.Get("active"); v != nil {
		if v.Kind != yamlite.Bool {
			c.typeIssue(path+".active", v.Line, "boolean", v)
		} else {
			out.Active = v.Bool
		}
	}
	return out
}

// ParseCompaniesFile parses and validates one data/companies file
// (companiesFileSchema). file names the file in errors.
func ParseCompaniesFile(file string, src []byte) ([]CompanyYAML, []string, error) {
	root, err := yamlite.Parse(file, src)
	if err != nil {
		return nil, nil, err
	}
	c := &checker{}
	var out []CompanyYAML
	var unknown []string
	if root.Kind != yamlite.Map {
		c.typeIssue("(root)", root.Line, "object", root)
		return nil, nil, &SchemaError{File: file, Issues: c.issues}
	}
	list := root.Get("companies")
	switch {
	case list == nil || list.Kind != yamlite.Seq:
		line := root.Line
		if list != nil {
			line = list.Line
		}
		c.typeIssue("companies", line, "array", list)
	case len(list.Items) == 0:
		c.add("companies", list.Line, "Too small: expected array to have >=1 items")
	default:
		for i, it := range list.Items {
			co := parseCompany(c, fmt.Sprintf("companies[%d]", i), it)
			co.File = file
			out = append(out, co)
			if it.Kind == yamlite.Map {
				for _, p := range it.Pairs {
					if !knownCompanyKeys[p.Key] {
						unknown = append(unknown, fmt.Sprintf("%s:%d: unknown key %q (ignored)", file, p.Line, p.Key))
					}
				}
			}
		}
	}
	for _, p := range root.Pairs {
		if p.Key != "companies" {
			unknown = append(unknown, fmt.Sprintf("%s:%d: unknown key %q (ignored)", file, p.Line, p.Key))
		}
	}
	if len(c.issues) > 0 {
		return nil, nil, &SchemaError{File: file, Issues: c.issues}
	}
	return out, unknown, nil
}

// ParseTier1 parses and validates data/curated/tier-1.yaml (tier1CuratedSchema).
func ParseTier1(file string, src []byte) ([]string, error) {
	root, err := yamlite.Parse(file, src)
	if err != nil {
		return nil, err
	}
	c := &checker{}
	var slugs []string
	if root.Kind != yamlite.Map {
		c.typeIssue("(root)", root.Line, "object", root)
		return nil, &SchemaError{File: file, Issues: c.issues}
	}
	list := root.Get("slugs")
	switch {
	case list == nil || list.Kind != yamlite.Seq:
		line := root.Line
		if list != nil {
			line = list.Line
		}
		c.typeIssue("slugs", line, "array", list)
	case len(list.Items) == 0:
		c.add("slugs", list.Line, "Too small: expected array to have >=1 items")
	default:
		for i, it := range list.Items {
			p := fmt.Sprintf("slugs[%d]", i)
			if s, ok := c.str(p, it.Line, it, false); ok {
				c.slug(p, it.Line, s)
				slugs = append(slugs, s)
			}
		}
	}
	if len(c.issues) > 0 {
		return nil, &SchemaError{File: file, Issues: c.issues}
	}
	return slugs, nil
}

// jsTrim is String.prototype.trim: JavaScript whitespace and line
// terminators (it includes U+FEFF and excludes U+0085, unlike Go).
func jsTrim(s string) string {
	return strings.TrimFunc(s, isJSSpace)
}

func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
