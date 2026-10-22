package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/syz51/golang-server-template/internal/config"
	"github.com/syz51/golang-server-template/internal/handler"
	"github.com/syz51/golang-server-template/internal/middleware"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Logger, os.Stderr)
	ctx, stop := shutdownContext()
	defer stop()
	sc := serverStartConfig(cfg)
	logger.Info("Server starting", "address", sc.Address)
	e := newServer(cfg)
	e.Logger = logger
	if err := startServer(ctx, e, sc); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Server exited")
}

func shutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func newLogger(cfg config.LoggerConfig, output io.Writer) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	options := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.New(slog.NewTextHandler(output, options))
	}
	return slog.New(slog.NewJSONHandler(output, options))
}

func serverStartConfig(cfg *config.Config) echo.StartConfig {
	return echo.StartConfig{
		Address:         net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: cfg.Server.ShutdownTimeout,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadHeaderTimeout = cfg.Server.ReadHeaderTimeout
			s.ReadTimeout = cfg.Server.ReadTimeout
			s.WriteTimeout = cfg.Server.WriteTimeout
			s.IdleTimeout = cfg.Server.IdleTimeout
			return nil
		},
	}
}

func newServer(cfg *config.Config) *echo.Echo {
	e := echo.New()
	e.Logger = newLogger(cfg.Logger, os.Stderr)

	// Add middleware
	e.Use(echomiddleware.RequestLogger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.RequestID())
	if len(cfg.Server.CORSAllowedOrigins) > 0 {
		e.Use(echomiddleware.CORS(cfg.Server.CORSAllowedOrigins...))
	}
	e.Use(bodyLimit(cfg.Server.MaxBodyBytes))
	e.Use(middleware.Config(cfg))

	// Initialize handlers
	h := handler.New(cfg)

	// Routes
	setupRoutes(e, h)
	return e
}

// Read the bounded body before binding: a JSON decoder may finish after one
// object without reading an oversized chunked tail. Echo's limiter bounds the
// allocation and preserves its status-coded 413 error.
func bodyLimit(limit int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return echomiddleware.BodyLimit(limit)(func(c *echo.Context) error {
			req := c.Request()
			body, err := io.ReadAll(req.Body)
			if err != nil {
				if echo.StatusCode(err) == http.StatusRequestEntityTooLarge {
					return err
				}
				return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload").Wrap(err)
			}
			req.Body = struct {
				io.Reader
				io.Closer
			}{bytes.NewReader(body), req.Body}
			return next(c)
		})
	}
}

func startServer(ctx context.Context, e *echo.Echo, sc echo.StartConfig) error {
	// Start waits for its shutdown goroutine, but reports shutdown errors only
	// through this callback. Preserve the application's failure exit on timeout.
	var shutdownErr error
	sc.OnShutdownError = func(err error) {
		shutdownErr = err
	}
	if err := sc.Start(ctx, e); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	if shutdownErr != nil {
		return fmt.Errorf("server forced to shutdown: %w", shutdownErr)
	}
	return nil
}

func setupRoutes(e *echo.Echo, h *handler.Handler) {
	// Health check
	e.GET("/health", h.Health)

	// API v1 group
	api := e.Group("/api/v1")

	// User routes
	users := api.Group("/users")
	users.POST("", h.CreateUser)
	users.GET("/:id", h.GetUser)
	users.PUT("/:id", h.UpdateUser)
	users.DELETE("/:id", h.DeleteUser)
	users.GET("", h.ListUsers)
}
