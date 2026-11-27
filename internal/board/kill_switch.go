package board

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

type KillSwitch struct {
	Jobs        map[string]struct{}
	Companies   map[string]struct{}
	Hosts       map[string]struct{}
	Maintenance bool
	Message     *string
}

type killCache struct {
	value     KillSwitch
	checkedAt time.Time
}

type killSwitchJSON struct {
	Jobs        json.RawMessage `json:"jobs"`
	Companies   json.RawMessage `json:"companies"`
	Hosts       json.RawMessage `json:"hosts"`
	Maintenance json.RawMessage `json:"maintenance"`
	Message     json.RawMessage `json:"message"`
}

func emptyKillSwitch() KillSwitch {
	return KillSwitch{
		Jobs:      make(map[string]struct{}),
		Companies: make(map[string]struct{}),
		Hosts:     make(map[string]struct{}),
	}
}

func parseStringSet(raw json.RawMessage, field string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("kill switch: %s must be a list of strings", field)
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out, nil
}

func ParseKillSwitch(data []byte) (KillSwitch, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return emptyKillSwitch(), nil
	}
	if len(trimmed) == 0 {
		return KillSwitch{}, fmt.Errorf("decode kill switch: unexpected end of JSON input")
	}
	if trimmed[0] != '{' {
		var value any
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return KillSwitch{}, fmt.Errorf("decode kill switch: %w", err)
		}
		return emptyKillSwitch(), nil
	}
	var raw killSwitchJSON
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return KillSwitch{}, fmt.Errorf("decode kill switch: %w", err)
	}
	jobs, err := parseStringSet(raw.Jobs, "jobs")
	if err != nil {
		return KillSwitch{}, err
	}
	companies, err := parseStringSet(raw.Companies, "companies")
	if err != nil {
		return KillSwitch{}, err
	}
	hosts, err := parseStringSet(raw.Hosts, "hosts")
	if err != nil {
		return KillSwitch{}, err
	}

	maintenance := false
	if len(raw.Maintenance) > 0 && string(raw.Maintenance) != "null" {
		if err := json.Unmarshal(raw.Maintenance, &maintenance); err != nil {
			return KillSwitch{}, fmt.Errorf("kill switch: maintenance must be true or false")
		}
	}
	var message *string
	if len(raw.Message) > 0 && string(raw.Message) != "null" {
		var value string
		if json.Unmarshal(raw.Message, &value) == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				value = truncateUTF16(value, 500)
				message = &value
			}
		}
	}
	return KillSwitch{
		Jobs:        jobs,
		Companies:   companies,
		Hosts:       hosts,
		Maintenance: maintenance,
		Message:     message,
	}, nil
}

// RefreshKillSwitch returns the copy currently in force. Failed refreshes keep
// the last successfully parsed copy and are retried after the refresh window.
func (s *Store) RefreshKillSwitch(ctx context.Context) KillSwitch {
	if s.killSource == "" {
		s.mu.RLock()
		value := s.kill.value
		s.mu.RUnlock()
		if value.Jobs == nil {
			return emptyKillSwitch()
		}
		return value
	}
	now := s.now()
	s.mu.RLock()
	cached := s.kill
	s.mu.RUnlock()
	if cached.value.Jobs != nil && now.Sub(cached.checkedAt) < s.killRefresh {
		return cached.value
	}

	s.killLoad.Lock()
	defer s.killLoad.Unlock()
	now = s.now()
	s.mu.RLock()
	cached = s.kill
	s.mu.RUnlock()
	if cached.value.Jobs != nil && now.Sub(cached.checkedAt) < s.killRefresh {
		return cached.value
	}

	requestCtx, cancel := context.WithTimeout(ctx, KillSwitchHTTPTimeout)
	defer cancel()
	body, err := s.readSource(requestCtx, s.killSource, maxAncillaryBytes)
	var value KillSwitch
	if err == nil {
		value, err = ParseKillSwitch(body)
	}
	s.mu.Lock()
	if err == nil {
		s.kill.value = value
	} else if s.kill.value.Jobs == nil {
		s.kill.value = emptyKillSwitch()
	}
	s.kill.checkedAt = now
	value = s.kill.value
	s.mu.Unlock()
	return value
}

func killed(companySlug, jobID, applyURL string, ks KillSwitch) bool {
	if _, ok := ks.Jobs[strings.ToLower(jobID)]; ok {
		return true
	}
	if _, ok := ks.Companies[strings.ToLower(companySlug)]; ok {
		return true
	}
	if len(ks.Hosts) == 0 {
		return false
	}
	u, err := url.Parse(applyURL)
	if err != nil || u.Hostname() == "" {
		return true
	}
	_, ok := ks.Hosts[strings.ToLower(u.Hostname())]
	return ok
}

func (s *Store) publishable(job model.DumpJob, ks KillSwitch) bool {
	if s.isDelisted(job.ID) || killed(job.Company.Slug, job.ID, job.ApplyURL, ks) {
		return false
	}
	return s.applyURLAllowed(job.Company.Slug, job.ApplyURL)
}
