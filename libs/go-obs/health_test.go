package obs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	return rec.Code, body
}

func TestHealthzAlways200(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHealth()
	h.Register("db", func(context.Context) error { return errors.New("down") })
	h.Mount(mux)
	if code, _ := get(t, mux, "/healthz"); code != 200 {
		t.Fatalf("healthz = %d", code)
	}
}

func TestReadyzReflectsDependencyAndRecovers(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHealth()
	var failing atomic.Bool
	failing.Store(true)
	h.Register("db", func(context.Context) error {
		if failing.Load() {
			return errors.New("db down")
		}
		return nil
	})
	h.Register("cache", func(context.Context) error { return nil })
	h.Mount(mux)

	code, body := get(t, mux, "/readyz")
	if code != http.StatusServiceUnavailable || body["status"] != "unavailable" {
		t.Fatalf("want 503 unavailable, got %d %v", code, body)
	}
	if c := body["checks"].(map[string]any); c["db"] != "db down" || c["cache"] != "ok" {
		t.Fatalf("checks = %v", c)
	}

	failing.Store(false)
	if code, body = get(t, mux, "/readyz"); code != 200 || body["status"] != "ok" {
		t.Fatalf("want 200 after recovery, got %d %v", code, body)
	}
}

func TestReadyzNoChecksIsReady(t *testing.T) {
	mux := http.NewServeMux()
	NewHealth().Mount(mux)
	if code, _ := get(t, mux, "/readyz"); code != 200 {
		t.Fatalf("readyz = %d", code)
	}
}
