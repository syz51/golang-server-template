package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/syz51/golang-server-template/internal/config"
	"github.com/syz51/golang-server-template/internal/model"
	"github.com/syz51/golang-server-template/internal/service"

	"github.com/labstack/echo/v5"
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
func (h *Handler) Health(c *echo.Context) error {
	response := model.HealthResponse{
		Status:    "ok",
		Service:   h.config.App.Name,
		Version:   h.config.App.Version,
		Timestamp: time.Now(),
	}

	return c.JSON(http.StatusOK, response)
}

// CreateUser creates a new user
func (h *Handler) CreateUser(c *echo.Context) error {
	var req model.CreateUserRequest

	if err := c.Bind(&req); err != nil {
		return bindingError(c, err)
	}

	if err := model.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "Validation failed",
			Details: map[string]interface{}{"validation_errors": model.GetValidationErrors(err)},
		})
	}

	user, err := h.userService.CreateUser(&req)
	if err != nil {
		return serviceError(c, err)
	}

	return c.JSON(http.StatusCreated, user)
}

// GetUser retrieves a user by ID
func (h *Handler) GetUser(c *echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	user, err := h.userService.GetUser(id)
	if err != nil {
		return serviceError(c, err)
	}

	return c.JSON(http.StatusOK, user)
}

// UpdateUser updates an existing user
func (h *Handler) UpdateUser(c *echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return bindingError(c, err)
	}

	if err := model.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "Validation failed",
			Details: map[string]interface{}{"validation_errors": model.GetValidationErrors(err)},
		})
	}

	user, err := h.userService.UpdateUser(id, &req)
	if err != nil {
		return serviceError(c, err)
	}

	return c.JSON(http.StatusOK, user)
}

// DeleteUser deletes a user by ID
func (h *Handler) DeleteUser(c *echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid user ID",
		})
	}

	if err := h.userService.DeleteUser(id); err != nil {
		return serviceError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// ListUsers returns a paginated list of users
func (h *Handler) ListUsers(c *echo.Context) error {
	query, err := url.ParseQuery(c.Request().URL.RawQuery)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid query parameters"})
	}
	page, perPage := 1, 10
	for name, target := range map[string]*int{"page": &page, "per_page": &perPage} {
		if values, present := query[name]; present {
			if len(values) != 1 {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid " + name})
			}
			value, err := strconv.Atoi(values[0])
			if err != nil {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid " + name})
			}
			*target = value
		}
	}

	response, err := h.userService.ListUsers(page, perPage)
	if err != nil {
		return serviceError(c, err)
	}

	return c.JSON(http.StatusOK, response)
}

// bindingError preserves transport errors raised while reading a limited body.
func bindingError(c *echo.Context, err error) error {
	status := echo.StatusCode(err)
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		status = http.StatusRequestEntityTooLarge
	}
	if status != http.StatusRequestEntityTooLarge && status != http.StatusUnsupportedMediaType {
		status = http.StatusBadRequest
	}
	message := "Invalid request payload"
	if status != http.StatusBadRequest {
		message = http.StatusText(status)
	}
	return c.JSON(status, model.ErrorResponse{Error: message})
}

func serviceError(c *echo.Context, err error) error {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrEmailExists):
		status = http.StatusConflict
	case errors.Is(err, service.ErrInvalidPagination):
		status = http.StatusBadRequest
	}
	return c.JSON(status, model.ErrorResponse{Error: err.Error()})
}
