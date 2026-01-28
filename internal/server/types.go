package server

import (
	"fmt"
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

// APIError represents an HTTP API error.
type APIError struct {
	Status  int    `json:"-"`
	Message string `json:"error"`
}

func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new APIError.
func NewAPIError(status int, msg string) *APIError {
	return &APIError{Status: status, Message: msg}
}

// ValidateTag validates a tag name.
func ValidateTag(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty tag")
	}
	if !tagPattern.MatchString(s) {
		return "", fmt.Errorf("invalid tag: %s", s)
	}
	return s, nil
}
