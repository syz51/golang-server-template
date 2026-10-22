package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/syz51/golang-server-template/internal/model"
)

// Health returns a liveness handler with the application's identity.
func Health(name, version string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return c.JSON(http.StatusOK, model.HealthResponse{
			Status: "ok", Service: name, Version: version, Timestamp: time.Now(),
		})
	}
}

// HTTPErrorHandler sends the public error contract. RequestLogger logs the
// original error; internal details must never be copied into the response.
func HTTPErrorHandler(c *echo.Context, err error) {
	if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
		return
	}
	code := http.StatusInternalServerError
	var statusCoder echo.HTTPStatusCoder
	if errors.As(err, &statusCoder) && statusCoder.StatusCode() != 0 {
		code = statusCoder.StatusCode()
	}
	message := http.StatusText(code)
	if httpError, ok := statusCoder.(*echo.HTTPError); ok && code < http.StatusInternalServerError && httpError.Message != "" {
		message = httpError.Message
	}
	var writeErr error
	if c.Request().Method == http.MethodHead {
		writeErr = c.NoContent(code)
	} else {
		writeErr = c.JSON(code, model.ErrorResponse{Error: message})
	}
	if writeErr != nil {
		c.Logger().Error("failed to send error response", "error", writeErr)
	}
}
