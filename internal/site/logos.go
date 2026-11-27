package site

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/logos"
)

// LogoPrefix is where the committed company logos (data/logos) are served,
// from this origin: a visitor's browser never asks a company site or a
// favicon service for an image.
const LogoPrefix = "/logos/"

// companyMark is the small square beside a company name: the company's logo
// when one is committed, else its initials.
type companyMark struct {
	Src      string // LogoPrefix URL with a content hash, or ""
	Initials string
}

var (
	logoOnce   sync.Once
	logoHashes map[string]string // slug -> short content hash
)

func logoFS() fs.FS {
	sub, err := fs.Sub(data.Logos, "logos")
	if err != nil {
		panic(err)
	}
	return sub
}

func loadLogos() {
	logoOnce.Do(func() {
		logoHashes = map[string]string{}
		entries, err := fs.ReadDir(logoFS(), ".")
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".png") {
				continue
			}
			b, err := fs.ReadFile(logoFS(), name)
			if err != nil {
				continue
			}
			sum := sha256.Sum256(b)
			logoHashes[strings.TrimSuffix(name, ".png")] = hex.EncodeToString(sum[:5])
		}
	})
}

// mark is the company mark for a canonical company slug.
func mark(slug, name string) companyMark {
	loadLogos()
	m := companyMark{Initials: initials(name)}
	if h, ok := logoHashes[slug]; ok {
		m.Src = LogoPrefix + logos.FileName(slug) + "?v=" + h
	}
	return m
}

// companyMark resolves merged or renamed slugs first, so an old dump row
// still finds its company's logo.
func (h *handler) companyMark(slug, name string) companyMark {
	return mark(h.store.CanonicalCompanySlug(slug), name)
}

// initials is one or two letters for the fallback mark: the first letters of
// the first two words, or the first two of a single word. A leading "The"
// and anything in parentheses are skipped.
func initials(name string) string {
	if i := strings.IndexByte(name, '('); i > 0 {
		name = name[:i]
	}
	words := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
	if len(words) > 1 && strings.EqualFold(words[0], "the") {
		words = words[1:]
	}
	var out []rune
	switch {
	case len(words) == 0:
		return "?"
	case len(words) == 1:
		for _, r := range words[0] {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				out = append(out, r)
			}
			if len(out) == 2 {
				break
			}
		}
	default:
		for _, w := range words[:2] {
			for _, r := range w {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					out = append(out, r)
					break
				}
			}
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return strings.ToUpper(string(out))
}

var logoName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.png$`)

// logo serves one committed logo. URLs carry ?v=<hash>, so a year of
// immutable caching is safe; a bare URL gets a day.
func (h *handler) logo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !logoName.MatchString(name) {
		h.notFound(w, r)
		return
	}
	b, err := fs.ReadFile(logoFS(), name)
	if err != nil {
		h.notFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	_, _ = w.Write(b)
}
