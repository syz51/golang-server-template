package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"github.com/your-org/your-project/internal/config"
	"github.com/your-org/your-project/internal/model"
)

func testServer() (*echo.Echo, *bytes.Buffer) {
	cfg := &config.Config{App: config.AppConfig{Name: "test-app", Version: "1.0.0"}}
	e := newServer(cfg)
	var logs bytes.Buffer
	e.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	return e, &logs
}

func request(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestServerUserRoutes(t *testing.T) {
	e, _ := testServer()

	rec := request(e, http.MethodGet, "/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var health model.HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &health))
	require.Equal(t, "test-app", health.Service)
	require.Equal(t, "1.0.0", health.Version)

	const userJSON = `{"email":"john@example.com","first_name":"John","last_name":"Doe","age":25}`
	rec = request(e, http.MethodPost, "/api/v1/users", userJSON)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var user model.User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &user))
	require.Equal(t, 1, user.ID)
	require.Equal(t, "john@example.com", user.Email)

	rec = request(e, http.MethodGet, "/api/v1/users/1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &user))
	require.Equal(t, "John", user.FirstName)

	rec = request(e, http.MethodPut, "/api/v1/users/1", `{"first_name":"Jane","status":"inactive"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &user))
	require.Equal(t, "Jane", user.FirstName)
	require.Equal(t, "inactive", user.Status)
	require.Equal(t, "john@example.com", user.Email)

	rec = request(e, http.MethodGet, "/api/v1/users?page=1&per_page=1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list model.UserListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Users, 1)
	require.Equal(t, model.MetaData{Page: 1, PerPage: 1, Total: 1, TotalPages: 1}, list.Meta)

	rec = request(e, http.MethodDelete, "/api/v1/users/1", "")
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
	rec = request(e, http.MethodGet, "/api/v1/users/1", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServerRequestErrors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		err    string
	}{
		{"malformed create", http.MethodPost, "/api/v1/users", `{"email":`, "Invalid request payload"},
		{"invalid create", http.MethodPost, "/api/v1/users", `{"email":"invalid"}`, "Validation failed"},
		{"malformed update", http.MethodPut, "/api/v1/users/1", `{"age":`, "Invalid request payload"},
		{"invalid update", http.MethodPut, "/api/v1/users/1", `{"age":-1}`, "Validation failed"},
		{"invalid get ID", http.MethodGet, "/api/v1/users/abc", "", "Invalid user ID"},
		{"invalid update ID", http.MethodPut, "/api/v1/users/abc", `{}`, "Invalid user ID"},
		{"invalid delete ID", http.MethodDelete, "/api/v1/users/abc", "", "Invalid user ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := testServer()
			rec := request(e, tt.method, tt.path, tt.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var response model.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Equal(t, tt.err, response.Error)
			if tt.err == "Validation failed" {
				require.NotEmpty(t, response.Details["validation_errors"])
			}
		})
	}
}

func TestServerMiddlewareErrorsAndRequestLogs(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		status  int
		message string
	}{
		{"not found", http.MethodGet, "/missing", http.StatusNotFound, "Not Found"},
		{"method not allowed", http.MethodPost, "/health", http.StatusMethodNotAllowed, "Method Not Allowed"},
		{"wrapped HTTP error", http.MethodGet, "/http-error", http.StatusTeapot, "brew elsewhere"},
		{"internal error", http.MethodGet, "/error", http.StatusInternalServerError, "Internal Server Error"},
		{"recovered panic", http.MethodGet, "/panic", http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, logs := testServer()
			e.GET("/http-error", func(c *echo.Context) error {
				return echo.NewHTTPError(http.StatusTeapot, "brew elsewhere").Wrap(errors.New("private detail"))
			})
			e.GET("/error", func(c *echo.Context) error { return errors.New("private detail") })
			e.GET("/panic", func(c *echo.Context) error { panic("private detail") })
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set(echo.HeaderOrigin, "https://example.com")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, tt.status, rec.Code)
			require.JSONEq(t, `{"message":"`+tt.message+`"}`, rec.Body.String())
			require.Equal(t, "*", rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
			requestID := rec.Header().Get(echo.HeaderXRequestID)
			require.NotEmpty(t, requestID)

			var entry struct {
				Message   string `json:"msg"`
				Method    string `json:"method"`
				URI       string `json:"uri"`
				Status    int    `json:"status"`
				RequestID string `json:"request_id"`
				BytesOut  int    `json:"bytes_out"`
			}
			require.NoError(t, json.Unmarshal(logs.Bytes(), &entry), logs.String())
			require.Equal(t, "REQUEST_ERROR", entry.Message)
			require.Equal(t, tt.method, entry.Method)
			require.Equal(t, tt.path, entry.URI)
			require.Equal(t, tt.status, entry.Status)
			require.Equal(t, requestID, entry.RequestID)
			require.Equal(t, rec.Body.Len(), entry.BytesOut)
		})
	}
}

func TestServerRequestIDAndPreflight(t *testing.T) {
	e, logs := testServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(echo.HeaderXRequestID, "client-request")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "client-request", rec.Header().Get(echo.HeaderXRequestID))
	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	require.Equal(t, "REQUEST", entry["msg"])
	require.Equal(t, float64(http.StatusOK), entry["status"])
	require.Equal(t, "client-request", entry["request_id"])

	req = httptest.NewRequest(http.MethodOptions, "/api/v1/users", nil)
	req.Header.Set(echo.HeaderOrigin, "https://example.com")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "*", rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
	require.Contains(t, rec.Header().Get(echo.HeaderAccessControlAllowMethods), http.MethodPost)
}

func TestStartServerShutdown(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		name := "drains active request"
		if timeout {
			name = "reports shutdown timeout"
		}
		t.Run(name, func(t *testing.T) {
			e, _ := testServer()
			entered := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			e.GET("/slow", func(c *echo.Context) error {
				close(entered)
				<-release
				return c.String(http.StatusOK, "finished")
			})
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			shutdownStarted := make(chan struct{})
			var server *http.Server
			sc := echo.StartConfig{
				Listener:        listener,
				HideBanner:      true,
				HidePort:        true,
				GracefulTimeout: 5 * time.Second,
				BeforeServeFunc: func(s *http.Server) error {
					server = s
					s.RegisterOnShutdown(func() { close(shutdownStarted) })
					return nil
				},
			}
			if timeout {
				sc.GracefulTimeout = time.Millisecond
			}
			done := make(chan error, 1)
			go func() { done <- startServer(ctx, e, sc) }()
			transport := &http.Transport{DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
			response := make(chan error, 1)
			go func() {
				resp, err := client.Get("http://" + listener.Addr().String() + "/slow")
				if err == nil {
					defer resp.Body.Close()
					body, readErr := io.ReadAll(resp.Body)
					err = readErr
					if err == nil && (resp.StatusCode != http.StatusOK || string(body) != "finished") {
						err = errors.New("active request did not finish successfully")
					}
				}
				response <- err
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not reach handler")
			}
			defer server.Close()
			cancel()
			select {
			case <-shutdownStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown did not start")
			}
			if timeout {
				select {
				case err := <-done:
					require.ErrorIs(t, err, context.DeadlineExceeded)
				case <-time.After(5 * time.Second):
					t.Fatal("shutdown timeout was not reported")
				}
			} else {
				select {
				case err := <-done:
					t.Fatalf("server returned before active request finished: %v", err)
				default:
				}
			}
			// A send releases the handler while keeping cleanup's close safe.
			release <- struct{}{}
			select {
			case err := <-response:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("active request did not finish")
			}
			if !timeout {
				select {
				case err := <-done:
					require.NoError(t, err)
				case <-time.After(5 * time.Second):
					t.Fatal("server did not finish graceful shutdown")
				}
			}
		})
	}
}

func TestStartServerStartupError(t *testing.T) {
	e, _ := testServer()
	err := startServer(context.Background(), e, echo.StartConfig{Address: "127.0.0.1:invalid"})
	require.ErrorContains(t, err, "failed to start server")
}
