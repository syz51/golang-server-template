package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-org/your-project/internal/config"
	"github.com/your-org/your-project/internal/model"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
	// Setup
	cfg := &config.Config{
		App: config.AppConfig{
			Name:    "test-app",
			Version: "1.0.0",
		},
	}
	handler := New(cfg)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test
	err := handler.Health(c)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response model.HealthResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "test-app", response.Service)
	assert.Equal(t, "1.0.0", response.Version)
}

func TestCreateUserHandler(t *testing.T) {
	// Setup
	cfg := &config.Config{
		App: config.AppConfig{
			Name:    "test-app",
			Version: "1.0.0",
		},
	}
	handler := New(cfg)

	e := echo.New()

	// Test data
	user := model.CreateUserRequest{
		Email:     "test@example.com",
		FirstName: "John",
		LastName:  "Doe",
		Age:       25,
		Phone:     "+1234567890",
	}

	jsonData, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBuffer(jsonData))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test
	err := handler.CreateUser(c)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var response model.User
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, response.Email)
	assert.Equal(t, user.FirstName, response.FirstName)
	assert.Equal(t, user.LastName, response.LastName)
	assert.Equal(t, user.Age, response.Age)
	assert.Equal(t, "active", response.Status)
}

func TestCreateUserHandlerValidationError(t *testing.T) {
	// Setup
	cfg := &config.Config{
		App: config.AppConfig{
			Name:    "test-app",
			Version: "1.0.0",
		},
	}
	handler := New(cfg)

	e := echo.New()

	// Test data with invalid email
	user := model.CreateUserRequest{
		Email:     "invalid-email",
		FirstName: "John",
		LastName:  "Doe",
		Age:       25,
	}

	jsonData, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBuffer(jsonData))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test
	err := handler.CreateUser(c)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response model.ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Validation failed", response.Error)
	assert.NotNil(t, response.Details)
}
