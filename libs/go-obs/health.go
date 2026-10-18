package obs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// CheckFunc reports whether a dependency is ready. A non-nil error means not ready.
type CheckFunc func(ctx context.Context) error

// Health serves /healthz (liveness) and /readyz (readiness with pluggable checks).
type Health struct {
	mu      sync.RWMutex
	checks  map[string]*check
	timeout time.Duration
	logger  *slog.Logger
}

type check struct {
	fn       CheckFunc
	inflight atomic.Bool
}

// NewHealth returns a Health with no dependency checks registered. Each
// readiness probe is bounded by a 2s timeout.
func NewHealth() *Health {
	return &Health{checks: map[string]*check{}, timeout: 2 * time.Second}
}

// SetLogger sets the logger that receives failed-check details (default slog.Default()).
func (h *Health) SetLogger(l *slog.Logger) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logger = l
}

// Register adds (or replaces) a named readiness check.
func (h *Health) Register(name string, fn CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = &check{fn: fn}
}

// Mount registers /healthz and /readyz on mux.
func (h *Health) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.Liveness)
	mux.HandleFunc("/readyz", h.Readiness)
}

// Liveness always reports 200 while the process can serve requests.
func (h *Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// Readiness runs every registered check concurrently; any failure or timeout
// yields 503. The timeout is enforced here, so a check that ignores its context
// still cannot delay the response. A check whose previous run is still hung is
// not started again (reported unavailable), which bounds leaked goroutines to
// one per check. The body carries only check names and "ok"/"unavailable";
// error details are logged, not returned.
func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	checks := make(map[string]*check, len(h.checks))
	for n, c := range h.checks {
		checks[n] = c
	}
	logger := h.logger
	h.mu.RUnlock()
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}
	out := make(chan result, len(checks)) // buffered: late finishers never block
	for n, c := range checks {
		if !c.inflight.CompareAndSwap(false, true) {
			out <- result{n, errors.New("previous run still in progress")}
			continue
		}
		go func() {
			defer c.inflight.Store(false)
			defer func() {
				if p := recover(); p != nil {
					out <- result{n, fmt.Errorf("panic: %v", p)}
				}
			}()
			out <- result{n, c.fn(ctx)}
		}()
	}

	results := make(map[string]string, len(checks))
	for n := range checks {
		results[n] = "unavailable"
	}
	status, code := "ok", http.StatusOK
collect:
	for pending := len(checks); pending > 0; pending-- {
		select {
		case res := <-out:
			if res.err != nil {
				logger.ErrorContext(r.Context(), "readiness check failed", "check", res.name, "error", res.err)
				continue
			}
			results[res.name] = "ok"
		case <-ctx.Done():
			for n, v := range results {
				if v != "ok" {
					logger.ErrorContext(r.Context(), "readiness check failed or timed out", "check", n, "error", ctx.Err())
				}
			}
			break collect
		}
	}
	for _, v := range results {
		if v != "ok" {
			status, code = "unavailable", http.StatusServiceUnavailable
		}
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
