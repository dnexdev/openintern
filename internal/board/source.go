package board

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

const (
	maxDumpBytes      = 128 << 20
	maxAncillaryBytes = 16 << 20
)

type dumpCache struct {
	value     *model.JobsDump
	checkedAt time.Time
	loadedAt  time.Time
}

type companiesCache struct {
	value     []model.DumpCompany
	available bool
	checkedAt time.Time
}

type healthCache struct {
	value     *HealthSnapshot
	checkedAt time.Time
}

// Store owns independent, guarded caches so tests and separately configured
// servers cannot leak state into one another.
type Store struct {
	dumpSource      string
	companiesSource string
	healthSource    string
	killSource      string
	client          *http.Client
	now             func() time.Time
	dumpRefresh     time.Duration
	killRefresh     time.Duration

	canonicalCompanySlug func(string) string
	jobAlias             func(string) string
	jobAliasByPrefix     func(string) string
	jobPath              func(string, string, string) string
	applyURLAllowed      func(string, string) bool
	isDelisted           func(string) bool
	isPublic             func(model.DumpJob, time.Time) bool

	mu        sync.RWMutex
	dump      dumpCache
	companies companiesCache
	health    healthCache
	kill      killCache

	dumpLoad      sync.Mutex
	companiesLoad sync.Mutex
	healthLoad    sync.Mutex
	killLoad      sync.Mutex
}

// NewStore creates a dump-backed store. It does not perform I/O until the
// first method call.
func NewStore(opts Options) *Store {
	dumpSource := strings.TrimSpace(opts.DumpSource)
	if dumpSource == "" {
		dumpSource = strings.TrimSpace(os.Getenv("JOBS_DUMP_URL"))
	}
	if dumpSource == "" {
		dumpSource = DefaultDumpURL
	}
	healthSource := strings.TrimSpace(opts.HealthSource)
	if healthSource == "" {
		healthSource = strings.TrimSpace(os.Getenv("HEALTH_SNAPSHOT_URL"))
	}
	if healthSource == "" {
		healthSource = siblingSource(dumpSource, "health.json")
	}
	killSource := strings.TrimSpace(opts.KillSwitchSource)
	if killSource == "" {
		killSource = strings.TrimSpace(os.Getenv("OPENINTERN_KILL_SWITCH_URL"))
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: HTTPTimeout}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	dumpRefresh := opts.DumpRefresh
	if dumpRefresh <= 0 {
		dumpRefresh = DumpRefreshInterval
	}
	killRefresh := opts.KillRefresh
	if killRefresh <= 0 {
		killRefresh = KillSwitchRefreshInterval
	}

	s := &Store{
		dumpSource:           dumpSource,
		companiesSource:      siblingSource(dumpSource, "companies.json"),
		healthSource:         healthSource,
		killSource:           killSource,
		client:               client,
		now:                  now,
		dumpRefresh:          dumpRefresh,
		killRefresh:          killRefresh,
		canonicalCompanySlug: opts.CanonicalCompanySlug,
		jobAlias:             opts.JobAlias,
		jobAliasByPrefix:     opts.JobAliasByPrefix,
		jobPath:              opts.JobPath,
		applyURLAllowed:      opts.ApplyURLAllowed,
		isDelisted:           opts.IsDelisted,
		isPublic:             opts.IsPublic,
	}
	s.fillDefaults()
	return s
}

func siblingSource(source, name string) string {
	u, err := url.Parse(source)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "file") {
		if u.Scheme == "file" {
			u.Path = filepath.ToSlash(filepath.Join(filepath.Dir(u.Path), name))
			return u.String()
		}
		u.Path = strings.TrimSuffix(u.Path, filepath.Base(u.Path)) + name
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	}
	return filepath.Join(filepath.Dir(source), name)
}

func (s *Store) readSource(ctx context.Context, source string, limit int64) ([]byte, error) {
	if source == "" {
		return nil, errors.New("source is not configured")
	}
	u, err := url.Parse(source)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		requestCtx, cancel := context.WithTimeout(ctx, HTTPTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, source, nil)
		if err != nil {
			return nil, fmt.Errorf("create GET request: %w", err)
		}
		res, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", source, err)
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			return nil, fmt.Errorf("GET %s: HTTP %d", source, res.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source, err)
		}
		if int64(len(body)) > limit {
			return nil, fmt.Errorf("read %s: response exceeds %d bytes", source, limit)
		}
		return body, nil
	}
	path := source
	if err == nil && u.Scheme == "file" {
		path = u.Path
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("read %s: file exceeds %d bytes", path, limit)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Store) jobsDump(ctx context.Context) (*model.JobsDump, error) {
	now := s.now()
	s.mu.RLock()
	cached := s.dump
	s.mu.RUnlock()
	if cached.value != nil && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, nil
	}

	s.dumpLoad.Lock()
	defer s.dumpLoad.Unlock()
	now = s.now()
	s.mu.RLock()
	cached = s.dump
	s.mu.RUnlock()
	if cached.value != nil && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, nil
	}

	body, err := s.readSource(ctx, s.dumpSource, maxDumpBytes)
	if err == nil {
		var dump model.JobsDump
		err = json.Unmarshal(body, &dump)
		if err != nil {
			err = fmt.Errorf("decode jobs dump: %w", err)
		} else if dump.Jobs == nil {
			err = errors.New("decode jobs dump: missing jobs array")
		} else {
			if dump.Count == 0 && len(dump.Jobs) > 0 {
				dump.Count = len(dump.Jobs)
			}
			s.mu.Lock()
			s.dump = dumpCache{value: &dump, checkedAt: now, loadedAt: now}
			s.mu.Unlock()
			return &dump, nil
		}
	}

	s.mu.Lock()
	s.dump.checkedAt = now
	fallback := s.dump.value
	s.mu.Unlock()
	if fallback != nil {
		return fallback, nil
	}
	return nil, err
}

func (s *Store) companiesDump(ctx context.Context) ([]model.DumpCompany, bool) {
	now := s.now()
	s.mu.RLock()
	cached := s.companies
	s.mu.RUnlock()
	if !cached.checkedAt.IsZero() && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, cached.available
	}

	s.companiesLoad.Lock()
	defer s.companiesLoad.Unlock()
	now = s.now()
	s.mu.RLock()
	cached = s.companies
	s.mu.RUnlock()
	if !cached.checkedAt.IsZero() && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, cached.available
	}

	var wrapper model.CompaniesDump
	body, err := s.readSource(ctx, s.companiesSource, maxAncillaryBytes)
	if err == nil {
		err = json.Unmarshal(body, &wrapper)
	}
	available := err == nil && wrapper.Companies != nil
	s.mu.Lock()
	if available {
		s.companies.value = wrapper.Companies
		s.companies.available = true
	} else if !s.companies.available {
		s.companies.value = nil
	}
	s.companies.checkedAt = now
	out := s.companies
	s.mu.Unlock()
	return out.value, out.available
}

func (s *Store) HealthSnapshot(ctx context.Context) (*HealthSnapshot, error) {
	now := s.now()
	s.mu.RLock()
	cached := s.health
	s.mu.RUnlock()
	if cached.value != nil && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, nil
	}

	s.healthLoad.Lock()
	defer s.healthLoad.Unlock()
	now = s.now()
	s.mu.RLock()
	cached = s.health
	s.mu.RUnlock()
	if cached.value != nil && now.Sub(cached.checkedAt) < s.dumpRefresh {
		return cached.value, nil
	}

	body, err := s.readSource(ctx, s.healthSource, maxAncillaryBytes)
	if err == nil {
		var snapshot HealthSnapshot
		err = json.Unmarshal(body, &snapshot)
		if err != nil {
			err = fmt.Errorf("decode health snapshot: %w", err)
		} else {
			var presence struct {
				Stats            json.RawMessage `json:"stats"`
				RecentIngestRuns json.RawMessage `json:"recent_ingest_runs"`
			}
			if json.Unmarshal(body, &presence) != nil || len(presence.Stats) == 0 || string(presence.Stats) == "null" || snapshot.RecentIngestRuns == nil {
				err = errors.New("decode health snapshot: missing stats or recent_ingest_runs")
			} else {
				s.mu.Lock()
				s.health = healthCache{value: &snapshot, checkedAt: now}
				s.mu.Unlock()
				return &snapshot, nil
			}
		}
	}

	s.mu.Lock()
	s.health.checkedAt = now
	fallback := s.health.value
	s.mu.Unlock()
	if fallback != nil {
		return fallback, nil
	}
	return nil, err
}
