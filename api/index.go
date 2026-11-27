// Package handler is the Vercel entry point: vercel.json rewrites every path
// to this function, which serves the whole site and API through the same
// handler `openintern serve` runs. Everything it serves (registry, templates,
// CSS, fonts) is compiled in; the job data comes from the daily dump at run
// time (JOBS_DUMP_URL), so the deployment needs no build step besides go build.
package handler

import (
	"net/http"
	"sync"

	"github.com/dnexdev/openintern/server"
)

// One handler per instance: it holds the in-memory dump, its hourly refresh
// and the per-instance rate-limit buckets, so it must outlive a request.
var site = sync.OnceValue(func() http.Handler { return server.Handler() })

// Handler serves one request. Vercel calls it with the original request URL
// (the rewrite does not change the path the handler sees).
func Handler(w http.ResponseWriter, r *http.Request) {
	site().ServeHTTP(w, r)
}
