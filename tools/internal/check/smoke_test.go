package check

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSmokeVerifiesReadBackNames(t *testing.T) {
	for _, tc := range []struct {
		name        string
		initialName string
		staleUpdate bool
		wantError   string
	}{
		{name: "persisted update", initialName: "Smoke"},
		{name: "stale update", initialName: "Smoke", staleUpdate: true, wantError: `first_name "Smoke", want "Updated"`},
		{name: "incorrect initial read", initialName: "Other", wantError: `first_name "Other", want "Smoke"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// State belongs to the server handler. Smoke issues requests sequentially.
			firstName := tc.initialName
			created, deleted := false, false
			finished := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				writeUser := func(name string) {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id": 1, "email": "smoke@example.com", "first_name": name,
					})
				}
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/health":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "ok", "service": "fixture", "version": "test", "timestamp": time.Now(),
					})
				case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users":
					body, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if len(body) > 1<<20 {
						w.WriteHeader(http.StatusRequestEntityTooLarge)
					} else if created {
						w.WriteHeader(http.StatusConflict)
					} else {
						created = true
						w.WriteHeader(http.StatusCreated)
						writeUser("Smoke")
					}
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users":
					_, _ = io.WriteString(w, `[]`)
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/1":
					if deleted {
						w.WriteHeader(http.StatusNotFound)
						finished <- struct{}{}
					} else {
						writeUser(firstName)
					}
				case r.Method == http.MethodPut && r.URL.Path == "/api/v1/users/1":
					if !tc.staleUpdate {
						firstName = "Updated"
					}
					// Even the stale fixture claims success in its PUT response.
					writeUser("Updated")
				case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/users/1":
					deleted = true
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			err := Smoke(server.URL)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("Smoke() = %v, want error containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Smoke() = %v", err)
			}
			select {
			case <-finished:
			default:
				t.Fatal("Smoke did not verify the deleted user returns 404")
			}
		})
	}
}
