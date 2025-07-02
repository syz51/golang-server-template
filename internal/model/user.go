package model

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// User represents a user in the system
type User struct {
	ID        int       `json:"id" validate:"-"`
	Email     string    `json:"email" validate:"required,email" example:"user@example.com"`
	FirstName string    `json:"first_name" validate:"required,min=2,max=50" example:"John"`
	LastName  string    `json:"last_name" validate:"required,min=2,max=50" example:"Doe"`
	Age       int       `json:"age" validate:"required,min=1,max=150" example:"25"`
	Phone     string    `json:"phone,omitempty" validate:"omitempty,e164" example:"+1234567890"`
	Status    string    `json:"status" validate:"required,oneof=active inactive suspended" example:"active"`
	CreatedAt time.Time `json:"created_at" validate:"-"`
	UpdatedAt time.Time `json:"updated_at" validate:"-"`
}

// CreateUserRequest represents the request payload for creating a user
type CreateUserRequest struct {
	Email     string `json:"email" validate:"required,email" example:"user@example.com"`
	FirstName string `json:"first_name" validate:"required,min=2,max=50" example:"John"`
	LastName  string `json:"last_name" validate:"required,min=2,max=50" example:"Doe"`
	Age       int    `json:"age" validate:"required,min=1,max=150" example:"25"`
	Phone     string `json:"phone,omitempty" validate:"omitempty,e164" example:"+1234567890"`
}

// UpdateUserRequest represents the request payload for updating a user
type UpdateUserRequest struct {
	Email     *string `json:"email,omitempty" validate:"omitempty,email" example:"user@example.com"`
	FirstName *string `json:"first_name,omitempty" validate:"omitempty,min=2,max=50" example:"John"`
	LastName  *string `json:"last_name,omitempty" validate:"omitempty,min=2,max=50" example:"Doe"`
	Age       *int    `json:"age,omitempty" validate:"omitempty,min=1,max=150" example:"25"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,e164" example:"+1234567890"`
	Status    *string `json:"status,omitempty" validate:"omitempty,oneof=active inactive suspended" example:"active"`
}

// UserListResponse represents the response for listing users
type UserListResponse struct {
	Users []User   `json:"users"`
	Meta  MetaData `json:"meta"`
}

// MetaData represents pagination metadata
type MetaData struct {
	Page       int `json:"page" example:"1"`
	PerPage    int `json:"per_page" example:"10"`
	Total      int `json:"total" example:"100"`
	TotalPages int `json:"total_pages" example:"10"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string                 `json:"error" example:"Validation failed"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status" example:"ok"`
	Service   string            `json:"service" example:"golang-server-template"`
	Version   string            `json:"version" example:"1.0.0"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// Validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ValidateStruct validates a struct using the validator tags
func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

// GetValidationErrors returns formatted validation errors
func GetValidationErrors(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			field := fieldError.Field()
			tag := fieldError.Tag()
			param := fieldError.Param()

			switch tag {
			case "required":
				errors[field] = "This field is required"
			case "email":
				errors[field] = "Must be a valid email address"
			case "min":
				errors[field] = "Must be at least " + param + " characters long"
			case "max":
				errors[field] = "Must be at most " + param + " characters long"
			case "oneof":
				errors[field] = "Must be one of: " + param
			case "e164":
				errors[field] = "Must be a valid phone number in E164 format"
			default:
				errors[field] = "Invalid value"
			}
		}
	}

	return errors
}
