package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/your-org/your-project/internal/config"
	"github.com/your-org/your-project/internal/handler"
	"github.com/your-org/your-project/internal/middleware"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Echo instance
	e := echo.New()

	// Configure Echo
	e.HideBanner = true
	e.HidePort = true

	// Add middleware
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.RequestID())
	e.Use(middleware.Config(cfg))

	// Initialize handlers
	h := handler.New(cfg)

	// Routes
	setupRoutes(e, h)

	// Start server
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		log.Printf("Server starting on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
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
