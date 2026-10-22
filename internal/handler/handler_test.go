package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"github.com/syz51/golang-server-template/internal/model"
)

func TestHealth(t *testing.T) {
	e := echo.New()
	before := time.Now()
	rec := httptest.NewRecorder()
	require.NoError(t, Health("test-app", "1.0.0")(e.NewContext(httptest.NewRequest(http.MethodGet, "/health", nil), rec)))
	require.Equal(t, http.StatusOK, rec.Code)
	var response model.HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "ok", response.Status)
	require.Equal(t, "test-app", response.Service)
	require.Equal(t, "1.0.0", response.Version)
	require.False(t, response.Timestamp.Before(before))
	require.False(t, response.Timestamp.After(time.Now()))
	var fields map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fields))
	require.Len(t, fields, 4)
}

type privateStatusError struct{}

func (privateStatusError) Error() string   { return "private detail" }
func (privateStatusError) StatusCode() int { return http.StatusUnauthorized }

func TestHTTPErrorHandler(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"plain internal error", errors.New("private detail"), 500, "Internal Server Error"},
		{"wrapped public HTTP error", fmt.Errorf("private wrapper: %w", echo.NewHTTPError(418, "brew elsewhere").Wrap(errors.New("private detail"))), 418, "brew elsewhere"},
		{"empty HTTP message", echo.NewHTTPError(400, ""), 400, "Bad Request"},
		{"private HTTP server error", echo.NewHTTPError(503, "private detail"), 503, "Service Unavailable"},
		{"framework error", echo.ErrNotFound, 404, "Not Found"},
		{"custom status coder", privateStatusError{}, 401, "Unauthorized"},
		{"zero status", echo.NewHTTPError(0, "private detail"), 500, "Internal Server Error"},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				e := echo.New()
				rec := httptest.NewRecorder()
				rec.Header().Set("WWW-Authenticate", "Bearer")
				c := e.NewContext(httptest.NewRequest(method, "/", nil), rec)
				HTTPErrorHandler(c, tc.err)
				require.Equal(t, tc.status, rec.Code)
				require.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
				if method == http.MethodHead {
					require.Empty(t, rec.Body.String())
				} else {
					require.Contains(t, rec.Header().Get(echo.HeaderContentType), echo.MIMEApplicationJSON)
					expected, err := json.Marshal(model.ErrorResponse{Error: tc.message})
					require.NoError(t, err)
					require.JSONEq(t, string(expected), rec.Body.String())
					require.NotContains(t, rec.Body.String(), "private")
				}
			})
		}
	}
}

func TestHTTPErrorHandlerCommittedResponse(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	require.NoError(t, c.String(http.StatusAccepted, "already sent"))
	HTTPErrorHandler(c, errors.New("private detail"))
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "already sent", rec.Body.String())
}
