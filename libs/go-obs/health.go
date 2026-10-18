package obs

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// CheckFunc reports whether a dependency is ready. A non-nil error means not ready.
type CheckFunc func(ctx context.Context) error

// Health serves /healthz (liveness) and /readyz (readiness with pluggable checks).
type Health struct {
	mu      sync.RWMutex
	checks  map[string]CheckFunc
	timeout time.Duration
}

// NewHealth returns a Health with no dependency checks registered.
func NewHealth() *Health {
	return &Health{checks: map[string]CheckFunc{}, timeout: 2 * time.Second}
}

// Register adds (or replaces) a named readiness check.
func (h *Health) Register(name string, check CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
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

// Readiness runs every registered check; any failure yields 503.
func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	names := make([]string, 0, len(h.checks))
	for n := range h.checks {
		names = append(names, n)
	}
	checks := make(map[string]CheckFunc, len(h.checks))
	for n, c := range h.checks {
		checks[n] = c
	}
	h.mu.RUnlock()
	sort.Strings(names)

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	status, code := "ok", http.StatusOK
	results := make(map[string]string, len(names))
	for _, n := range names {
		if err := checks[n](ctx); err != nil {
			results[n] = err.Error()
			status, code = "unavailable", http.StatusServiceUnavailable
		} else {
			results[n] = "ok"
		}
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
