package site

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
)

const (
	defaultTitle       = "OpenIntern — free open-source tech internships"
	defaultDescription = "Free tech internship board with no account, a public API, and daily dumps. Open source (Apache-2.0). Never paywalled."
	ogDescription      = "Free tech internship board with no account, a public API, and daily dumps. Open source. Never paywalled."
	twitterDescription = "Free tech internship board. No account. Public API and daily dumps. Open source."

	// Pages that change with the query string are never cached by shared
	// caches; the others revalidate on every request, as before.
	cacheDynamic = "private, no-cache, no-store, max-age=0, must-revalidate"
	cachePublic  = "public, max-age=0, must-revalidate"
)

// meta is the <head> of one page.
type meta struct {
	Title       string
	Description string
	Canonical   string // absolute URL, or empty for none
	NoIndex     bool
	// Open Graph overrides; empty fields fall back to the site defaults.
	OGTitle       string
	OGDescription string
	OGType        string
	OGURL         string
}

type stale struct {
	Age string
}

// view is what layout.html receives.
type view struct {
	Meta      meta
	Nav       string // "jobs" or "docs" marks the current nav item
	Analytics bool
	Stale     *stale
	Aliases   template.JS // job-alias map for the Applied toggle, when the page has toggles
	Data      any
}

type templates struct {
	pages map[string]*template.Template
}

var (
	assetOnce   sync.Once
	assetHashes map[string]string
)

// assetURL returns the public URL of an embedded asset with a content hash, so
// the immutable cache headers on AssetPrefix stay correct across deploys.
func assetURL(name string) string {
	assetOnce.Do(func() {
		assetHashes = map[string]string{}
		_ = fs.WalkDir(Static(), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(Static(), p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			assetHashes[p] = hex.EncodeToString(sum[:5])
			return nil
		})
	})
	if h, ok := assetHashes[name]; ok {
		return AssetPrefix + name + "?v=" + h
	}
	return AssetPrefix + name
}

var funcs = template.FuncMap{
	"asset": assetURL,
	"siteURL": func(p string) string {
		return SiteURL + p
	},
	"plural": func(n int, one, many string) string {
		if n == 1 {
			return one
		}
		return many
	},
	"num": formatInt,
	"add": func(a, b int) int { return a + b },
	"pair": func(id, title string) struct{ ID, Title string } {
		return struct{ ID, Title string }{id, title}
	},
}

func mustTemplates() *templates {
	base := template.Must(template.New("").Funcs(funcs).ParseFS(templateFiles, "templates/partials/*.html"))
	pages, err := fs.Glob(templateFiles, "templates/*.html")
	if err != nil {
		panic(err)
	}
	t := &templates{pages: map[string]*template.Template{}}
	for _, p := range pages {
		clone := template.Must(base.Clone())
		template.Must(clone.ParseFS(templateFiles, p))
		t.pages[strings.TrimSuffix(path.Base(p), ".html")] = clone
	}
	return t
}

func (h *handler) render(w http.ResponseWriter, status int, cache, name string, v view) {
	v.Analytics = h.opts.analytics
	t, ok := h.tmpl.pages[name]
	if !ok {
		http.Error(w, "unknown page", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", v); err != nil {
		log.Printf("site: render %s: %v", name, err)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal error. Please try again."))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cache)
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// formatInt prints n with thousands separators, as toLocaleString("en-US").
func formatInt(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// truncateUTF16 cuts s to at most n UTF-16 code units, as String.slice does
// in the reference implementation, without splitting a surrogate pair.
func truncateUTF16(s string, n int) string {
	units := 0
	for i, r := range s {
		w := len(utf16.Encode([]rune{r}))
		if units+w > n {
			return s[:i]
		}
		units += w
	}
	return s
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
