package check

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSmokeSkeletonContract(t *testing.T) {
	for _, tc := range []struct {
		name, defect, wantError string
	}{
		{name: "skeleton"},
		{"invalid health", "health", "invalid health fields"},
		{"missing service", "service", "invalid health fields"},
		{"missing version", "version", "invalid health fields"},
		{"missing timestamp", "timestamp", "invalid health fields"},
		{"demo still shipped", "demo", "status 200, want 404"},
		{"fallback succeeds", "fallback", "status 200, want 404"},
		{"method accepted", "method", "status 200, want 405"},
		{"framework envelope", "envelope", "error-only public message"},
		{"extra error details", "details", "error-only public message"},
		{"empty error", "empty", "error-only public message"},
		{"invalid error JSON", "json", "invalid JSON error envelope"},
		{"wrong content type", "content-type", "application/json"},
		{"missing request ID", "missing-id", "missing request ID"},
		{"replaced request ID", "replaced-id", "request ID was not preserved"},
		{"known length bypass", "known-limit", "status 200, want 413"},
		{"missing route body bypass", "chunked-limit", "status 404, want 413"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			sawChunkedMissing := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// An independent HTTP fixture: bound raw bytes before route selection,
				// without using the application's Echo handlers or middleware.
				body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
				if err != nil {
					t.Errorf("read request: %v", err)
					return
				}
				oversized := len(body) > 1<<20
				chunked := len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked"
				if r.URL.Path == "/does-not-exist" && oversized && chunked {
					mu.Lock()
					sawChunkedMissing = true
					mu.Unlock()
				}
				status := http.StatusNotFound
				if r.URL.Path == "/health" {
					status = http.StatusMethodNotAllowed
					if r.Method == http.MethodGet {
						status = http.StatusOK
					}
				}
				if oversized {
					status = http.StatusRequestEntityTooLarge
				}
				switch tc.defect {
				case "demo":
					if r.URL.Path == "/api/v1/users" {
						status = http.StatusOK
					}
				case "fallback":
					if r.URL.Path == "/" {
						status = http.StatusOK
					}
				case "method":
					if r.URL.Path == "/health" && r.Method == http.MethodPost {
						status = http.StatusOK
					}
				case "known-limit":
					if oversized && !chunked {
						status = http.StatusOK
					}
				case "chunked-limit":
					if oversized && chunked {
						status = http.StatusNotFound
					}
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				requestID := r.Header.Get("X-Request-ID")
				if requestID == "" {
					requestID = "fixture-generated-id"
				}
				w.Header().Set("X-Request-ID", requestID)
				// Keep readiness valid so negative cases fail at their specific assertion.
				if r.URL.Path == "/" {
					switch tc.defect {
					case "content-type":
						w.Header().Set("Content-Type", "text/plain")
					case "missing-id":
						w.Header().Del("X-Request-ID")
					}
				}
				if tc.defect == "replaced-id" && r.Header.Get("X-Request-ID") != "" {
					w.Header().Set("X-Request-ID", "replacement")
				}
				var payload any = map[string]string{"error": http.StatusText(status)}
				if status == http.StatusOK {
					healthStatus := "ok"
					if tc.defect == "health" {
						healthStatus = "unhealthy"
					}
					health := map[string]any{"status": healthStatus, "service": "fixture", "version": "test", "timestamp": time.Now()}
					switch tc.defect {
					case "service", "version", "timestamp":
						delete(health, tc.defect)
					}
					payload = health
				} else if r.URL.Path == "/" {
					switch tc.defect {
					case "envelope":
						payload = map[string]string{"message": "Not Found"}
					case "details":
						payload = map[string]string{"error": "Not Found", "details": "private"}
					case "empty":
						payload = map[string]string{"error": ""}
					}
				}
				w.WriteHeader(status)
				if tc.defect == "json" && r.URL.Path == "/" {
					_, _ = io.WriteString(w, "not JSON")
					return
				}
				_ = json.NewEncoder(w).Encode(payload)
			}))
			defer server.Close()
			err := Smoke(server.URL)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("Smoke() = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Smoke() = %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if !sawChunkedMissing {
				t.Fatal("Smoke did not send an oversized chunked body to a missing route")
			}
		})
	}
}
