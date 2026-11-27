package web

import (
	"io/fs"
	"net/http"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/site"
)

// registerSite mounts the page routes; APIs, metadata and assets stay here.
func registerSite(mux *http.ServeMux, store *board.Store) { site.Register(mux, store) }

// defaultStatic is the site's embedded asset tree, served under /assets/.
func defaultStatic() fs.FS { return site.Static() }
