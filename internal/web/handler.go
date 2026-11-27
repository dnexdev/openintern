// Package web assembles the OpenIntern website and public HTTP API.
package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/board"
)

type Options struct {
	Store        *board.Store
	Board        board.Options
	Now          func() time.Time
	RateLimit    *int64
	StaticFS     fs.FS
	RegisterSite func(mux *http.ServeMux, store *board.Store)
	BaseURL      string
	Maintenance  *bool
}

type server struct {
	store       *board.Store
	now         func() time.Time
	limiter     *rateLimiter
	maintenance *bool
	baseURL     string
}

// Handler returns the production handler. Passing one Options value supplies
// local dump paths and deterministic seams for tests.
func Handler(options ...Options) http.Handler {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	store := opts.Store
	if store == nil {
		boardOptions := opts.Board
		if boardOptions.Now == nil {
			boardOptions.Now = now
		}
		store = board.NewStore(boardOptions)
	}
	limit := environmentRateLimit()
	if opts.RateLimit != nil {
		limit = *opts.RateLimit
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://openintern.dev"
	}
	s := &server{
		store:       store,
		now:         now,
		limiter:     newRateLimiter(limit, now),
		maintenance: opts.Maintenance,
		baseURL:     baseURL,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.jobAPI)
	mux.HandleFunc("GET /api/v1/jobs", s.jobsAPI)
	mux.HandleFunc("GET /api/v1/companies", s.companiesAPI)
	mux.HandleFunc("GET /api/v1/health", s.healthAPI)
	mux.HandleFunc("GET /api/v1/status", s.statusAPI)
	mux.HandleFunc("GET /robots.txt", s.robots)
	mux.HandleFunc("GET /sitemap.xml", s.sitemap)
	if opts.StaticFS != nil {
		mux.Handle("GET /assets/", immutableFiles("/assets/", opts.StaticFS))
	}
	register := opts.RegisterSite
	if register == nil {
		register = registerSite
	}
	register(mux, store)
	return s.maintenanceGate(mux)
}

func immutableFiles(prefix string, files fs.FS) http.Handler {
	handler := http.StripPrefix(prefix, http.FileServerFS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		handler.ServeHTTP(w, r)
	})
}

func (s *server) maintenanceEnabled() bool {
	if s.maintenance != nil {
		return *s.maintenance
	}
	return strings.TrimSpace(os.Getenv("OPENINTERN_MAINTENANCE")) == "1"
}

func maintenanceEscaped(value string) string {
	replacer := strings.NewReplacer(
		"&", "&#38;",
		"<", "&#60;",
		">", "&#62;",
		"\"", "&#34;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func maintenanceExempt(path string) bool {
	return path == "/api/v1/status" ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/_next/static/") ||
		strings.HasPrefix(path, "/_next/image/") ||
		path == "/favicon.ico" || path == "/icon" || path == "/apple-icon"
}

func (s *server) maintenanceGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maintenanceExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		killSwitch := s.store.RefreshKillSwitch(r.Context())
		if !s.maintenanceEnabled() && !killSwitch.Maintenance {
			next.ServeHTTP(w, r)
			return
		}
		message := "OpenIntern is down for maintenance. Please check back soon."
		if killSwitch.Message != nil {
			message = *killSwitch.Message
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "600")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusServiceUnavailable, maintenanceResponse{Error: "Maintenance", Message: message})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex"><title>Maintenance · OpenIntern</title></head><body style="font-family:system-ui,sans-serif;max-width:36rem;margin:4rem auto;padding:0 1rem;line-height:1.5"><h1>Down for maintenance</h1><p>%s</p></body></html>`, maintenanceEscaped(message))
	})
}
