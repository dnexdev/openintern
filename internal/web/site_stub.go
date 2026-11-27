package web

import (
	"net/http"

	"github.com/dnexdev/openintern/internal/board"
)

// Kept in one file so the site package can replace this no-op registration as
// soon as its Register function lands.
func registerSite(*http.ServeMux, *board.Store) {}
