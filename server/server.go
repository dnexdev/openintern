// Package server exposes the site handler to entry points built outside this
// module's internal tree: Vercel compiles api/index.go as a separate module,
// and Go forbids that module from importing internal packages directly.
package server

import (
	"net/http"

	"github.com/dnexdev/openintern/internal/web"
)

// Handler is web.Handler with production settings.
func Handler() http.Handler { return web.Handler() }
