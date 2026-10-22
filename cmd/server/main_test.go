package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"github.com/syz51/golang-server-template/internal/config"
	"github.com/syz51/golang-server-template/internal/model"
)

func testServer() (*echo.Echo, *bytes.Buffer) {
	cfg := runtimeTestConfig()
	cfg.Server.CORSAllowedOrigins = []string{"*"}
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

func TestServerRoutes(t *testing.T) {
	e, _ := testServer()
	rec := request(e, http.MethodGet, "/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var health model.HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &health))
	require.Equal(t, "ok", health.Status)
	require.Equal(t, "test-app", health.Service)
	require.Equal(t, "1.0.0", health.Version)
	require.False(t, health.Timestamp.IsZero())
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users"},
		{http.MethodPost, "/api/v1/users"},
		{http.MethodGet, "/api/v1/users/1"},
		{http.MethodPut, "/api/v1/users/1"},
		{http.MethodDelete, "/api/v1/users/1"},
	} {
		rec := request(e, route.method, route.path, "")
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.JSONEq(t, `{"error":"Not Found"}`, rec.Body.String())
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
			if tt.status == http.StatusMethodNotAllowed {
				require.Contains(t, rec.Header().Get(echo.HeaderAllow), http.MethodGet)
			}
			require.JSONEq(t, `{"error":"`+tt.message+`"}`, rec.Body.String())
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

	e.POST("/preflight", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	req = httptest.NewRequest(http.MethodOptions, "/preflight", nil)
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

func runtimeTestConfig() *config.Config {
	return &config.Config{
		App:    config.AppConfig{Name: "test-app", Version: "1.0.0", Environment: "development"},
		Logger: config.LoggerConfig{Level: "info", Format: "json"},
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080, MaxBodyBytes: 1048576, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, ShutdownTimeout: 30 * time.Second},
	}
}

func TestConfiguredLogger(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		for _, level := range []string{"debug", "info", "warn", "error"} {
			t.Run(format+"/"+level, func(t *testing.T) {
				var logs bytes.Buffer
				logger := newLogger(config.LoggerConfig{Level: level, Format: format}, &logs)
				logger.Debug("debug-entry")
				logger.Info("info-entry")
				logger.Warn("warn-entry")
				logger.Error("error-entry")
				require.Equal(t, level == "debug", strings.Contains(logs.String(), "debug-entry"))
				require.Equal(t, level == "debug" || level == "info", strings.Contains(logs.String(), "info-entry"))
				require.Equal(t, level != "error", strings.Contains(logs.String(), "warn-entry"))
				require.Contains(t, logs.String(), "error-entry")
				for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
					if format == "json" {
						require.True(t, json.Valid([]byte(line)), line)
					} else {
						require.Contains(t, line, "level=")
						require.Contains(t, line, "msg=")
					}
				}
				cfg := runtimeTestConfig()
				cfg.Logger = config.LoggerConfig{Level: level, Format: format}
				e := newServer(cfg)
				require.Equal(t, level == "debug", e.Logger.Enabled(context.Background(), slog.LevelDebug))
				require.Equal(t, level != "error", e.Logger.Enabled(context.Background(), slog.LevelWarn))
				e.Logger = logger
				logs.Reset()
				request(e, http.MethodGet, "/health", "")
				require.Equal(t, level == "debug" || level == "info", strings.Contains(logs.String(), "REQUEST"))
			})
		}
	}
}

func TestServerBodyLimit(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		t.Run(fmt.Sprintf("chunked=%v", chunked), func(t *testing.T) {
			cfg := runtimeTestConfig()
			cfg.Server.MaxBodyBytes = 1024
			e := newServer(cfg)
			var logs bytes.Buffer
			e.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
			e.POST("/body", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
			for _, body := range []string{strings.Repeat("x", 2048), `{}` + strings.Repeat(" ", 2048)} {
				logs.Reset()
				req := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				if chunked {
					req.ContentLength = -1
					req.TransferEncoding = []string{"chunked"}
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
				require.JSONEq(t, `{"error":"Request Entity Too Large"}`, rec.Body.String())
				require.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
				var entry struct {
					Status    int    `json:"status"`
					RequestID string `json:"request_id"`
				}
				require.NoError(t, json.Unmarshal(logs.Bytes(), &entry), logs.String())
				require.Equal(t, http.StatusRequestEntityTooLarge, entry.Status)
				require.Equal(t, rec.Header().Get(echo.HeaderXRequestID), entry.RequestID)
			}
		})
	}
}

func TestServerCORSDisabledByDefault(t *testing.T) {
	e := newServer(runtimeTestConfig())
	e.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(echo.HeaderOrigin, "https://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
}

func TestServerStartConfig(t *testing.T) {
	cfg := runtimeTestConfig()
	cfg.Server.Host = "::1"
	cfg.Server.ReadHeaderTimeout = time.Second
	cfg.Server.ReadTimeout = 2 * time.Second
	cfg.Server.WriteTimeout = 3 * time.Second
	cfg.Server.IdleTimeout = 4 * time.Second
	cfg.Server.ShutdownTimeout = 5 * time.Second
	sc := serverStartConfig(cfg)
	require.Equal(t, "[::1]:8080", sc.Address)
	require.Equal(t, 5*time.Second, sc.GracefulTimeout)
	var server http.Server
	require.NoError(t, sc.BeforeServeFunc(&server))
	require.Equal(t, time.Second, server.ReadHeaderTimeout)
	require.Equal(t, 2*time.Second, server.ReadTimeout)
	require.Equal(t, 3*time.Second, server.WriteTimeout)
	require.Equal(t, 4*time.Second, server.IdleTimeout)
}

// Run the real signal registration and HTTP lifecycle in a separate process so
// signals cannot terminate the test runner or be intercepted by its other tests.
func TestSignalServerHelper(t *testing.T) {
	if os.Getenv("GO_RUNTIME_SIGNAL_HELPER") != "1" {
		return
	}
	cfg := runtimeTestConfig()
	e := newServer(cfg)
	e.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	e.GET("/slow", func(c *echo.Context) error {
		fmt.Println("entered")
		var release [1]byte
		if _, err := io.ReadFull(os.Stdin, release[:]); err != nil {
			return err
		}
		return c.String(http.StatusOK, "finished")
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := shutdownContext()
	defer stop()
	sc := serverStartConfig(cfg)
	sc.Listener = listener
	before := sc.BeforeServeFunc
	sc.BeforeServeFunc = func(s *http.Server) error {
		s.RegisterOnShutdown(func() { fmt.Println("shutdown") })
		if err := before(s); err != nil {
			return err
		}
		fmt.Println(listener.Addr().String())
		return nil
	}
	if err := startServer(ctx, e, sc); err != nil {
		t.Fatal(err)
	}
}

func TestSignalsDrainActiveRequests(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("real Unix signal tests support Linux and macOS")
	}
	for _, sig := range []os.Signal{syscall.SIGTERM, os.Interrupt} {
		t.Run(sig.String(), func(t *testing.T) {
			exe, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.Command(exe, "-test.run=^TestSignalServerHelper$")
			cmd.Env = append(os.Environ(), "GO_RUNTIME_SIGNAL_HELPER=1")
			stdout, err := cmd.StdoutPipe()
			require.NoError(t, err)
			stdin, err := cmd.StdinPipe()
			require.NoError(t, err)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			require.NoError(t, cmd.Start())
			t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill() })
			lines := make(chan string, 10)
			go func() {
				defer close(lines)
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					lines <- scanner.Text()
				}
			}()
			nextLine := func() string {
				t.Helper()
				select {
				case line, ok := <-lines:
					require.True(t, ok, "helper exited early")
					return line
				case <-time.After(10 * time.Second):
					t.Fatal("helper did not report lifecycle event")
					return ""
				}
			}
			addr := nextLine()
			transport := &http.Transport{DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
			response := make(chan error, 1)
			go func() {
				resp, err := client.Get("http://" + addr + "/slow")
				if err == nil {
					defer resp.Body.Close()
					body, readErr := io.ReadAll(resp.Body)
					err = readErr
					if err == nil && (resp.StatusCode != http.StatusOK || string(body) != "finished") {
						err = fmt.Errorf("unexpected response %d %q", resp.StatusCode, body)
					}
				}
				response <- err
			}()
			require.Equal(t, "entered", nextLine())
			require.NoError(t, cmd.Process.Signal(sig))
			require.Equal(t, "shutdown", nextLine())
			// Shutdown has started with a live request still blocked in the helper.
			require.NoError(t, cmd.Process.Signal(syscall.Signal(0)))
			_, err = stdin.Write([]byte{1})
			require.NoError(t, err)
			select {
			case err := <-response:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("active request did not drain")
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				require.NoError(t, err, stderr.String())
			case <-time.After(10 * time.Second):
				t.Fatal("helper did not exit after draining")
			}
		})
	}
}

func TestServerMalformedBodyOverTCP(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, tc := range []struct {
			name, headers, body string
		}{
			{"truncated content length", "Content-Length: 100\r\n", `{}`},
			{"invalid chunk size", "Transfer-Encoding: chunked\r\n", "invalid\r\n{}\r\n0\r\n\r\n"},
			{"invalid chunk terminator", "Transfer-Encoding: chunked\r\n", "2\r\n{}XX0\r\n\r\n"},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				e, _ := testServer()
				e.Match([]string{http.MethodPost, http.MethodPut}, "/body", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
				server := httptest.NewServer(e)
				defer server.Close()
				conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 5*time.Second)
				require.NoError(t, err)
				defer conn.Close()
				require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
				path := "/body"
				_, err = fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nConnection: close\r\n%s\r\n%s", method, path, tc.headers, tc.body)
				require.NoError(t, err)
				// Signal end of input while retaining the read side for the response.
				require.NoError(t, conn.(*net.TCPConn).CloseWrite())
				response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
				require.NoError(t, err)
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, response.StatusCode, string(body))
				require.JSONEq(t, `{"error":"Invalid request payload"}`, string(body))
				require.NotEmpty(t, response.Header.Get(echo.HeaderXRequestID))
			})
		}
	}
}

func TestBodyLimitBoundaryOverHTTP(t *testing.T) {
	cfg := runtimeTestConfig()
	cfg.Server.MaxBodyBytes = 1024
	e := newServer(cfg)
	e.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	e.POST("/body", func(c *echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		return c.String(http.StatusOK, string(body))
	})
	server := httptest.NewServer(e)
	defer server.Close()
	for _, chunked := range []bool{false, true} {
		for _, size := range []int{1024, 1025} {
			t.Run(fmt.Sprintf("chunked=%v/bytes=%d", chunked, size), func(t *testing.T) {
				req, err := http.NewRequest(http.MethodPost, server.URL+"/body", strings.NewReader(strings.Repeat("x", size)))
				require.NoError(t, err)
				if chunked {
					req.ContentLength = -1
					req.TransferEncoding = []string{"chunked"}
				}
				response, err := server.Client().Do(req)
				require.NoError(t, err)
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				if size == 1024 {
					require.Equal(t, http.StatusOK, response.StatusCode)
					require.Len(t, body, size)
				} else {
					require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode)
					require.JSONEq(t, `{"error":"Request Entity Too Large"}`, string(body))
				}
			})
		}
	}
}
