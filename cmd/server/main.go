package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/your-org/your-project/internal/config"
	"github.com/your-org/your-project/internal/handler"
	"github.com/your-org/your-project/internal/middleware"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	if err := startServer(ctx, newServer(cfg), echo.StartConfig{
		Address:         addr,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 30 * time.Second,
	}); err != nil {
		log.Fatal(err)
	}
	log.Println("Server exited")
}

func newServer(cfg *config.Config) *echo.Echo {
	e := echo.New()

	// Add middleware
	e.Use(echomiddleware.RequestLogger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORS("*"))
	e.Use(echomiddleware.RequestID())
	e.Use(middleware.Config(cfg))

	// Initialize handlers
	h := handler.New(cfg)

	// Routes
	setupRoutes(e, h)
	return e
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
