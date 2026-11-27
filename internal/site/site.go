// Package site renders the OpenIntern web pages: home, the job board, job
// detail, API docs, ingest health, privacy, the comparison page, errors and
// icons. The look is specified in DESIGN.md at the repository root; every page
// is built from the tokens and components described there.
//
// The package registers page routes only. internal/web owns the API, robots,
// sitemap, maintenance mode and serves Static() under AssetPrefix.
package site

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/board"
)

// AssetPrefix is the URL path under which internal/web must serve Static().
// Asset URLs carry a content hash query, so the immutable cache headers
// internal/web sets on that prefix are safe.
const AssetPrefix = "/assets/"

// SiteURL is the canonical origin used in canonical links and Open Graph tags.
const SiteURL = "https://openintern.dev"

//go:embed templates
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

// Static returns the embedded assets (CSS, JS, fonts, icons, the Open Graph
// image) rooted so that "site.css" is served at AssetPrefix+"site.css".
func Static() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// reader is the part of *board.Store the pages use; tests run the real store
// over a fixture dump.
type reader interface {
	Query(context.Context, board.FamilyQuery) (board.FamilyPage, error)
	Job(context.Context, string) (*board.JobDetail, board.DataSource, error)
	JobByCompanyAndIDPrefix(context.Context, string, string) (*board.JobDetail, board.DataSource, error)
	CompanyOptions(context.Context) ([]board.CompanyOption, board.DataSource, error)
	CorpusStats(context.Context) (board.CorpusStats, error)
	HeroPreview(context.Context) (board.HeroPreview, error)
	HealthSnapshot(context.Context) (*board.HealthSnapshot, error)
	DumpStatus(context.Context) (board.DumpStatus, error)
	StaleData(context.Context) board.StaleState
	ParseBoardSearchParams(url.Values) board.BoardSearch
	CanonicalCompanySlug(string) string
	JobAlias(string) string
	JobAliasByPrefix(string) string
}

type options struct {
	now       func() time.Time
	analytics bool
}

type handler struct {
	store reader
	opts  options
	tmpl  *templates
}

// Register adds every page route to mux. Patterns are method-qualified so
// they never shadow the API, robots, sitemap or asset routes of internal/web.
func Register(mux *http.ServeMux, store *board.Store) {
	register(mux, store, options{
		now: time.Now,
		// Vercel Web Analytics: a same-origin script, no cookies, page views
		// only. Enable Analytics on the Vercel project, then set
		// OPENINTERN_ANALYTICS=vercel.
		analytics: strings.TrimSpace(os.Getenv("OPENINTERN_ANALYTICS")) == "vercel",
	})
}

func register(mux *http.ServeMux, store reader, opts options) *handler {
	if opts.now == nil {
		opts.now = time.Now
	}
	h := &handler{store: store, opts: opts, tmpl: mustTemplates()}
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /jobs", h.jobs)
	mux.HandleFunc("GET /jobs/{company}", h.legacyJob)
	mux.HandleFunc("GET /jobs/{company}/{jobSlug}", h.jobDetail)
	mux.HandleFunc("GET /docs", h.docs)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /privacy", h.privacy)
	mux.HandleFunc("GET /vs/intern-insider", h.vsInternInsider)
	mux.HandleFunc("GET /icon.svg", h.iconSVG)
	mux.HandleFunc("GET /favicon.ico", h.favicon)
	mux.HandleFunc("GET /opengraph-image", h.ogImage)
	mux.HandleFunc("GET /twitter-image", h.ogImage)
	mux.HandleFunc("/", h.notFound)
	return h
}
