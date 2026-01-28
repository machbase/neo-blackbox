package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"blackbox-backend/internal/db"

	"github.com/gin-gonic/gin"
)

const videoStreamIndex = 0

// Handler handles HTTP API requests.
type Handler struct {
	db      *db.Machbase
	dataDir string

	prefixCache map[string]string
	mu          sync.RWMutex
}

// NewHandler creates a new Handler.
func NewHandler(machbase *db.Machbase, dataDir string) *Handler {
	return &Handler{
		db:          machbase,
		dataDir:     dataDir,
		prefixCache: make(map[string]string),
	}
}

func (h *Handler) requireTag(c *gin.Context, key string) (string, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		err := NewAPIError(http.StatusBadRequest, fmt.Sprintf("missing required parameter: %s", key))
		h.writeError(c, err)
		return "", err
	}

	tag, err := ValidateTag(raw)
	if err != nil {
		apiErr := NewAPIError(http.StatusBadRequest, err.Error())
		h.writeError(c, apiErr)
		return "", apiErr
	}

	return tag, nil
}

func (h *Handler) writeError(c *gin.Context, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.Status, gin.H{"error": apiErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
