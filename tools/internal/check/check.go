// Package check contains stdlib-only delivery smoke checks shared by the tools.
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func Smoke(base string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	request := func(method, path, body string, status int) ([]byte, error) {
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if err != nil {
			return nil, err
		}
		if res.StatusCode != status {
			return nil, fmt.Errorf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, status, data)
		}
		return data, nil
	}
	deadline := time.Now().Add(30 * time.Second)
	var health []byte
	for {
		data, err := request("GET", "/health", "", 200)
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
		return err
	}
	if h.Status != "ok" || h.Service == "" || h.Version == "" || h.Timestamp.IsZero() {
		return fmt.Errorf("invalid health fields: %s", health)
	}
	body := `{"email":"smoke@example.com","first_name":"Smoke","last_name":"Tester","age":25}`
	data, err := request("POST", "/api/v1/users", body, 201)
	if err != nil {
		return err
	}
	var user struct{ ID int }
	if err = json.Unmarshal(data, &user); err != nil {
		return err
	}
	if user.ID <= 0 {
		return fmt.Errorf("invalid created user: %s", data)
	}
	path := fmt.Sprintf("/api/v1/users/%d", user.ID)
	for _, tc := range []struct {
		method, path, body string
		status             int
		firstName          string
	}{
		{"GET", path, "", 200, "Smoke"}, {"GET", "/api/v1/users", "", 200, ""},
		{"POST", "/api/v1/users", body, 409, ""},
		{"PUT", path, `{"first_name":"Updated"}`, 200, "Updated"},
		{"GET", path, "", 200, "Updated"},
		{"PUT", "/api/v1/users/2147483647", `{"first_name":"Missing"}`, 404, ""},
		{"POST", "/api/v1/users", `{"padding":"` + strings.Repeat("x", (1<<20)+1) + `"}`, 413, ""},
		{"DELETE", path, "", 204, ""}, {"GET", path, "", 404, ""},
	} {
		data, err := request(tc.method, tc.path, tc.body, tc.status)
		if err != nil {
			return err
		}
		if tc.status == 200 && tc.path == path {
			var got struct {
				ID        int    `json:"id"`
				Email     string `json:"email"`
				FirstName string `json:"first_name"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				return err
			}
			if got.ID != user.ID || got.Email != "smoke@example.com" || got.FirstName == "" {
				return fmt.Errorf("invalid user response: %s", data)
			}
			if got.FirstName != tc.firstName {
				return fmt.Errorf("%s %s: first_name %q, want %q: %s", tc.method, tc.path, got.FirstName, tc.firstName, data)
			}
		}
	}
	fmt.Println("health and CRUD smoke passed")
	return nil
}
