package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Approval is one entry of data/expected-delta.json: it lets one expected
// swing through the state and dump gates until its date.
type Approval struct {
	Scope  string // "total", "ats:<name>" or "company:<slug>"
	Until  string // YYYY-MM-DD, inclusive
	Reason string
	// valid is false when an entry's fields are not strings; the gates then
	// treat it as no approval, as the JavaScript comparisons did.
	valid bool
}

// ExpectedDelta is data/expected-delta.json.
type ExpectedDelta struct {
	Approvals []Approval
}

// LoadExpectedDelta reads expected-delta.json from a data/-shaped FS. A
// missing file means no approvals, as in the gate scripts.
func LoadExpectedDelta(fsys fs.FS) (ExpectedDelta, error) {
	b, err := fs.ReadFile(fsys, ExpectedDeltaPath)
	if errors.Is(err, fs.ErrNotExist) {
		return ExpectedDelta{}, nil
	}
	if err != nil {
		return ExpectedDelta{}, err
	}
	return ParseExpectedDelta(b)
}

// ParseExpectedDelta parses the file's JSON.
func ParseExpectedDelta(b []byte) (ExpectedDelta, error) {
	v, err := jsjson.Parse(jsjson.DecodeUTF8(b))
	if err != nil {
		return ExpectedDelta{}, fmt.Errorf("%s: %w", ExpectedDeltaPath, err)
	}
	obj, ok := v.(jsjson.Object)
	if !ok {
		return ExpectedDelta{}, fmt.Errorf("%s: not a JSON object", ExpectedDeltaPath)
	}
	raw, _ := obj.Get("approvals")
	list, ok := raw.([]any)
	if !ok {
		return ExpectedDelta{}, fmt.Errorf("%s: approvals is not a list", ExpectedDeltaPath)
	}
	var out ExpectedDelta
	for _, e := range list {
		o, _ := e.(jsjson.Object)
		scope, sok := getString(o, "scope")
		until, uok := getString(o, "until")
		reason, rok := getString(o, "reason")
		out.Approvals = append(out.Approvals, Approval{Scope: scope, Until: until, Reason: reason, valid: sok && uok && rok})
	}
	return out, nil
}

func getString(o jsjson.Object, key string) (string, bool) {
	v, _ := o.Get(key)
	s, ok := v.(string)
	return s, ok
}

// Approved reports whether scope has an approval that runs until today or
// later (dates compared as strings, YYYY-MM-DD) and gives a reason.
func (e ExpectedDelta) Approved(scope, today string) bool {
	for _, a := range e.Approvals {
		if a.valid && a.Scope == scope && jsjson.CompareCodeUnits(a.Until, today) >= 0 && strings.TrimFunc(a.Reason, isJSSpace) != "" {
			return true
		}
	}
	return false
}
