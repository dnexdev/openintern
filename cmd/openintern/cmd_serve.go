package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/web"
)

func init() {
	register("serve", "serve the website and public API", runServe)
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := flags.String("addr", ":3000", "HTTP listen address")
	dump := flags.String("dump", "", "jobs dump URL or local file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	handler := web.Handler(web.Options{Board: board.Options{DumpSource: *dump}})
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	fmt.Printf("OpenIntern listening on %s\n", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
