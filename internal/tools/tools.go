// Package tools holds the maintenance commands that grow and repair the
// company registry: gap-report (which boards the community lists know about
// that we do not), discover-accelerators (YC and a16z portfolio boards),
// recover-tokens (find working tokens for inactive companies) and
// validate-tokens (the CI check that every board a PR touches answers).
//
// Each command mirrors the TypeScript script of the same name: same flags,
// output and exit codes. All network access goes through internal/fetch, so
// every request is honest (it identifies as OpenIntern, no forged headers).
package tools

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Env is everything a command touches outside its arguments. Tests fill it
// with buffers, a fixed clock and a temporary registry.
type Env struct {
	Stdout, Stderr io.Writer
	// DataDir is the registry directory (the repository's data/); the
	// commands read and write DataDir/companies.
	DataDir string
	// WorkDir resolves relative file arguments; "" is the process directory.
	WorkDir string
	Now     func() time.Time
	Getenv  func(string) string
	// Git runs git with args in dir and returns its stdout (validate-tokens
	// reads the base revision of a changed file). Nil runs the git binary.
	Git func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// DefaultEnv is the real process environment with the registry at dataDir.
func DefaultEnv(dataDir string) Env {
	return Env{Stdout: os.Stdout, Stderr: os.Stderr, DataDir: dataDir, Now: time.Now, Getenv: os.Getenv}
}

func (e Env) companiesDir() string { return filepath.Join(e.DataDir, "companies") }

func (e Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

func (e Env) getenv(k string) string {
	if e.Getenv == nil {
		return os.Getenv(k)
	}
	return e.Getenv(k)
}

func (e Env) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if e.Git != nil {
		return e.Git(ctx, dir, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// abs resolves p against WorkDir.
func (e Env) abs(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if e.WorkDir != "" {
		return filepath.Join(e.WorkDir, p)
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// isoDate is `new Date().toISOString().slice(0, 10)`.
func isoDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

// isoTime is `new Date().toISOString()`.
func isoTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// fetchText reads a document with one honest request (the scripts made one
// plain fetch, no retries). A failure reads as the TypeScript error message
// ("HTTP 404 for <url>").
func fetchText(ctx context.Context, url, accept string) (string, error) {
	res, err := fetch.Request(ctx, url, fetch.Options{Attempts: 1, Headers: map[string]string{"Accept": accept}})
	if err != nil {
		return "", failureMessage(err)
	}
	return decodeBody(res.Body), nil
}

func failureMessage(err error) error {
	var f *fetch.Failure
	if errors.As(err, &f) {
		return errors.New(f.Message)
	}
	return err
}

// decodeBody is Response.text(): lossy UTF-8 with a leading BOM dropped.
func decodeBody(b []byte) string {
	return strings.TrimPrefix(jsjson.DecodeUTF8(b), "\uFEFF")
}

// probeOK is probeAtsBoard(...).ok.
func probeOK(ctx context.Context, atsName, token string) bool {
	return ats.ProbeATSBoard(ctx, atsName, token).OK
}

// inChunks runs fn over items size at a time, each chunk concurrently, and
// returns the results in item order: `for (chunk) await Promise.all(...)`.
// after is called with each finished chunk's results before the next starts.
func inChunks[T, R any](ctx context.Context, items []T, size int, fn func(context.Context, T) R, after func([]R)) []R {
	var out []R
	for i := 0; i < len(items); i += size {
		chunk := items[i:min(i+size, len(items))]
		res, _ := ats.MapConcurrent(ctx, chunk, len(chunk), func(ctx context.Context, it T) (R, error) {
			return fn(ctx, it), nil
		})
		if after != nil {
			after(res)
		}
		out = append(out, res...)
	}
	return out
}

// --- JavaScript value semantics the scripts' argument handling relies on ---

var jsDecimal = regexp.MustCompile(`^[+-]?(?:\d+\.?\d*(?:[eE][+-]?\d+)?|\.\d+(?:[eE][+-]?\d+)?)$`)

// jsNumber is Number(s) for a string.
func jsNumber(s string) float64 {
	s = htmlx.TrimJS(s)
	switch s {
	case "":
		return 0
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			v := 0.0
			for _, c := range strings.ToLower(s[2:]) {
				d := strings.IndexRune("0123456789abcdef"[:base], c)
				if d < 0 {
					return math.NaN()
				}
				v = v*float64(base) + float64(d)
			}
			return v
		}
	}
	if !jsDecimal.MatchString(s) {
		return math.NaN()
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		var ne *strconv.NumError
		if errors.As(err, &ne) && errors.Is(ne.Err, strconv.ErrRange) {
			return f // ±Inf or 0, as JavaScript rounds
		}
		return math.NaN()
	}
	return f
}

// orDefault is `n || def` for a number.
func orDefault(n, def float64) float64 {
	if n == 0 || math.IsNaN(n) {
		return def
	}
	return n
}

// sliceEnd is the end index Array.prototype.slice(0, end) uses.
func sliceEnd(end float64, n int) int {
	if math.IsNaN(end) {
		return 0
	}
	if math.IsInf(end, 1) {
		return n
	}
	if math.IsInf(end, -1) {
		return 0
	}
	e := math.Trunc(end)
	if e < 0 {
		return int(math.Max(float64(n)+e, 0))
	}
	return int(math.Min(e, float64(n)))
}

func sliceTo[T any](s []T, end float64) []T { return s[:sliceEnd(end, len(s))] }

// jsString formats a number as template literals do.
func jsString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return string(jsjson.AppendNumber(nil, f))
}

// jsonNumber is a number as JSON.stringify writes it (non-finite → null).
func jsonNumber(f float64) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return f
}

// argValue is `args.find(a => a.startsWith(prefix))?.split("=")[1]`.
func argValue(args []string, prefix string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			parts := strings.Split(a, "=")
			if len(parts) < 2 {
				return "", true // split("=")[1] is undefined; Number(undefined) is NaN
			}
			return parts[1], true
		}
	}
	return "", false
}

func hasArg(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

// numberArg is Number(args.find(prefix).split("=")[1]); a missing value
// after "=" is undefined, which is NaN.
func numberArg(args []string, prefix string) (float64, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			parts := strings.Split(a, "=")
			if len(parts) < 2 {
				return math.NaN(), true
			}
			return jsNumber(parts[1]), true
		}
	}
	return 0, false
}

// jsonQuote is JSON.stringify(s).
func jsonQuote(s string) string { return string(jsjson.AppendString(nil, s)) }

// len16 is String.prototype.length.
func len16(s string) int { return jsjson.Len16(s) }
