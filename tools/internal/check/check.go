// Package check contains stdlib-only delivery smoke checks shared by the tools.
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func Run(dir string, env []string, name string, args ...string) error {
	fmt.Printf("+ %s %s\n", name, strings.Join(args, " "))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// Cleanup also removes read-only directories created by the Go module cache.
func Cleanup(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(path, 0700)
		}
		return nil
	})
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintln(os.Stderr, "cleanup:", err)
	}
}

// Smoke checks the shipped skeleton with the default 1 MiB request limit.
// It needs no application routes or test-only endpoints.
func Smoke(base string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	request := func(method, path, body, requestID string, status int, chunked bool) ([]byte, error) {
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
		if chunked {
			req.ContentLength = -1
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if err != nil {
			return nil, err
		}
		fail := func(message string) ([]byte, error) { return nil, fmt.Errorf("%s %s: %s", method, path, message) }
		if res.StatusCode != status {
			return fail(fmt.Sprintf("status %d, want %d: %s", res.StatusCode, status, data))
		}
		mediaType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			return fail("expected application/json response")
		}
		gotID := res.Header.Get("X-Request-ID")
		if strings.TrimSpace(gotID) == "" {
			return fail("missing request ID")
		}
		if requestID != "" && gotID != requestID {
			return fail("request ID was not preserved")
		}
		if status >= 400 {
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fail("invalid JSON error envelope")
			}
			var message string
			if len(envelope) != 1 || json.Unmarshal(envelope["error"], &message) != nil || strings.TrimSpace(message) == "" {
				return fail(fmt.Sprintf("expected error-only public message: %s", data))
			}
		}
		return data, nil
	}
	deadline := time.Now().Add(30 * time.Second)
	var health []byte
	for {
		data, err := request(http.MethodGet, "/health", "", "", http.StatusOK, false)
		if err == nil {
			health = data
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server readiness: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var h struct {
		Status, Service, Version string
		Timestamp                time.Time
	}
	if err := json.Unmarshal(health, &h); err != nil {
		return fmt.Errorf("health JSON: %w", err)
	}
	if h.Status != "ok" || strings.TrimSpace(h.Service) == "" || strings.TrimSpace(h.Version) == "" || h.Timestamp.IsZero() {
		return fmt.Errorf("invalid health fields: %s", health)
	}
	oversized := strings.Repeat("x", (1<<20)+1)
	for _, tc := range []struct {
		method, path, body, requestID string
		status                        int
		chunked                       bool
	}{
		{http.MethodGet, "/health", "", "smoke-health", http.StatusOK, false},
		{http.MethodGet, "/", "", "", http.StatusNotFound, false},
		{http.MethodGet, "/does-not-exist", "", "smoke-missing", http.StatusNotFound, false},
		{http.MethodGet, "/api/v1/users", "", "", http.StatusNotFound, false},
		{http.MethodPost, "/api/v1/users", `{}`, "", http.StatusNotFound, false},
		{http.MethodGet, "/api/v1/users/1", "", "", http.StatusNotFound, false},
		{http.MethodPost, "/health", "", "smoke-method", http.StatusMethodNotAllowed, false},
		{http.MethodGet, "/health", oversized, "smoke-limit", http.StatusRequestEntityTooLarge, false},
		{http.MethodPost, "/does-not-exist", oversized, "smoke-chunked", http.StatusRequestEntityTooLarge, true},
	} {
		if _, err := request(tc.method, tc.path, tc.body, tc.requestID, tc.status, tc.chunked); err != nil {
			return err
		}
	}
	fmt.Println("skeleton health, routes, errors, request IDs and body limits passed")
	return nil
}
