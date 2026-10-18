package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	if c := body["checks"].(map[string]any); c["db"] != "unavailable" || c["cache"] != "ok" {
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

// A check that ignores its context must still yield 503 at the timeout, not block or return 200.
func TestReadyzEnforcesTimeoutForContextIgnoringCheck(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h := NewHealth()
	h.timeout = 100 * time.Millisecond
	h.Register("hung", func(context.Context) error { <-release; return nil })
	h.Register("fine", func(context.Context) error { return nil })
	mux := http.NewServeMux()
	h.Mount(mux)

	start := time.Now()
	code, body := get(t, mux, "/readyz")
	if el := time.Since(start); el > time.Second {
		t.Fatalf("readyz took %v, want ~100ms", el)
	}
	c := body["checks"].(map[string]any)
	if code != http.StatusServiceUnavailable || c["hung"] != "unavailable" || c["fine"] != "ok" {
		t.Fatalf("got %d %v", code, body)
	}

	// While the hung run is still in flight, a second probe must not spawn another goroutine.
	code, _ = get(t, mux, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("second probe = %d", code)
	}
}

func TestReadyzRecoversAfterHungCheckReturns(t *testing.T) {
	var slow atomic.Bool
	slow.Store(true)
	release := make(chan struct{})
	h := NewHealth()
	h.timeout = 50 * time.Millisecond
	h.Register("db", func(context.Context) error {
		if slow.Load() {
			<-release
		}
		return nil
	})
	mux := http.NewServeMux()
	h.Mount(mux)
	if code, _ := get(t, mux, "/readyz"); code != 503 {
		t.Fatalf("want 503, got %d", code)
	}
	slow.Store(false)
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if code, _ := get(t, mux, "/readyz"); code == 200 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("never recovered")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReadyzDoesNotLeakErrorTextButLogsIt(t *testing.T) {
	var buf bytes.Buffer
	h := NewHealth()
	h.SetLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	h.Register("db", func(context.Context) error { return errors.New("dial postgres://user:pw@10.0.0.5/db") })
	rec := httptest.NewRecorder()
	h.Readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if strings.Contains(rec.Body.String(), "postgres") || strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("body leaks error: %s", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "10.0.0.5") || !strings.Contains(buf.String(), `"check":"db"`) {
		t.Fatalf("details not logged: %s", buf.String())
	}
}

func TestReadyzPanickingCheckIsFailureAndDefaultLogger(t *testing.T) {
	h := NewHealth()
	h.Register("boom", func(context.Context) error { panic("x") })
	mux := http.NewServeMux()
	h.Mount(mux)
	if code, _ := get(t, mux, "/readyz"); code != 503 {
		t.Fatalf("want 503, got %d", code)
	}
}

func TestReadyzConcurrentRegisterAndProbe(t *testing.T) {
	h := NewHealth()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); h.Register("a", func(context.Context) error { return nil }) }()
		go func() {
			defer wg.Done()
			h.Readiness(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))
		}()
	}
	wg.Wait()
}
