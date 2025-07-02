package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/your-org/your-project/internal/config"
	"github.com/your-org/your-project/internal/model"
	"github.com/your-org/your-project/internal/service"

	"github.com/labstack/echo/v4"
)

// Handler contains all the handlers
type Handler struct {
	config      *config.Config
	userService *service.UserService
}

// New creates a new handler instance
func New(cfg *config.Config) *Handler {
	return &Handler{
		config:      cfg,
		userService: service.NewUserService(),
	}
}

// Health returns the health status of the service
func (h *Handler) Health(c echo.Context) error {
	response := model.HealthResponse{
		Status:    "ok",
		Service:   h.config.App.Name,
		Version:   h.config.App.Version,
		Timestamp: time.Now(),
		Checks: map[string]string{
			"database": "ok", // In a real app, you'd check database connectivity
			"memory":   "ok",
		},
	}

	return c.JSON(http.StatusOK, response)
}

// CreateUser creates a new user
func (h *Handler) CreateUser(c echo.Context) error {
	var req model.CreateUserRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid request payload",
		})
	}

	if err := model.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "Validation failed",
			Details: map[string]interface{}{"validation_errors": model.GetValidationErrors(err)},
		})
	}

	user, err := h.userService.CreateUser(&req)
	if err != nil {
		return c.JSON(http.StatusConflict, model.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, user)
}

// GetUser retrieves a user by ID
func (h *Handler) GetUser(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	user, err := h.userService.GetUser(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, user)
}

// UpdateUser updates an existing user
func (h *Handler) UpdateUser(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid request payload",
		})
	}

	if err := model.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "Validation failed",
			Details: map[string]interface{}{"validation_errors": model.GetValidationErrors(err)},
		})
	}

	user, err := h.userService.UpdateUser(id, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "user not found" {
			status = http.StatusNotFound
		} else if err.Error() == "email already exists" {
			status = http.StatusConflict
		}

		return c.JSON(status, model.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, user)
}

// DeleteUser deletes a user by ID
func (h *Handler) DeleteUser(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	if err := h.userService.DeleteUser(id); err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// ListUsers returns a paginated list of users
func (h *Handler) ListUsers(c echo.Context) error {
	// Parse query parameters
	pageParam := c.QueryParam("page")
	perPageParam := c.QueryParam("per_page")

	page := 1
	perPage := 10

	if pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	if perPageParam != "" {
		if pp, err := strconv.Atoi(perPageParam); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	response, err := h.userService.ListUsers(page, perPage)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, response)
}
