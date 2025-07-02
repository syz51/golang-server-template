package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/your-org/your-project/internal/config"
)

// Config middleware injects the config into the context
func Config(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("config", cfg)
			return next(c)
		}
	}
}

// GetConfig retrieves the config from the context
func GetConfig(c echo.Context) *config.Config {
	return c.Get("config").(*config.Config)
}
