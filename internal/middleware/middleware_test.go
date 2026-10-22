package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"github.com/your-org/your-project/internal/config"
)

func TestConfigPassesContextAndError(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Name: "test-app"}}
	wantErr := errors.New("handler failure")
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	called := false
	err := Config(cfg)(func(got *echo.Context) error {
		called = true
		require.Same(t, c, got)
		require.Same(t, cfg, GetConfig(got))
		return wantErr
	})(c)
	require.True(t, called)
	require.ErrorIs(t, err, wantErr)
}
