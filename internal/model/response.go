package model

import "time"

// ErrorResponse contains a public error message.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse describes the running application's liveness and identity.
type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}
