package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetCameras handles GET /api/cameras
func (h *Handler) GetCameras(c *gin.Context) {
	cameras, err := h.db.GetCameras(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cameras": cameras})
}

// GetTimeRange handles GET /api/get_time_range
func (h *Handler) GetTimeRange(c *gin.Context) {
	camera, err := h.requireTag(c, "tagname")
	if err != nil {
		return
	}

	result, err := h.db.GetTimeRange(c.Request.Context(), camera)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetChunkInfo handles GET /api/get_chunk_info
func (h *Handler) GetChunkInfo(c *gin.Context) {
	camera, err := h.requireTag(c, "tagname")
	if err != nil {
		return
	}

	timeParam := strings.TrimSpace(c.Query("time"))
	if timeParam == "" {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "missing required parameter: time"))
		return
	}

	t, err := ParseTimeToken(timeParam)
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, err.Error()))
		return
	}

	info, err := h.db.GetChunkInfo(c.Request.Context(), camera, t)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if info == nil {
		h.writeError(c, NewAPIError(http.StatusNotFound, fmt.Sprintf("chunk not found: camera=%s time=%s", camera, timeParam)))
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetChunk handles GET /api/v_get_chunk
func (h *Handler) GetChunk(c *gin.Context) {
	camera, err := h.requireTag(c, "tagname")
	if err != nil {
		return
	}

	timeParam := c.DefaultQuery("time", "0")

	var data []byte
	if timeParam == "0" || strings.EqualFold(timeParam, "init") {
		data, err = h.readInitFile(camera)
	} else {
		t, parseErr := ParseTimeToken(timeParam)
		if parseErr != nil {
			h.writeError(c, NewAPIError(http.StatusBadRequest, parseErr.Error()))
			return
		}

		info, dbErr := h.db.GetChunkInfo(c.Request.Context(), camera, t)
		if dbErr != nil {
			h.writeError(c, dbErr)
			return
		}
		if info == nil {
			h.writeError(c, NewAPIError(http.StatusNotFound, fmt.Sprintf("chunk not found: camera=%s time=%s", camera, timeParam)))
			return
		}

		data, err = h.readChunkFile(c.Request.Context(), camera, info.Sign)
	}

	if err != nil {
		h.writeError(c, err)
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", data)
}

// GetCameraRollup handles GET /api/get_camera_rollup_info
func (h *Handler) GetCameraRollup(c *gin.Context) {
	camera, err := h.requireTag(c, "tagname")
	if err != nil {
		return
	}

	minutes, err := strconv.Atoi(c.DefaultQuery("minutes", "1"))
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "minutes must be an integer"))
		return
	}

	startStr := strings.TrimSpace(c.Query("start_time"))
	endStr := strings.TrimSpace(c.Query("end_time"))
	if startStr == "" || endStr == "" {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "missing required parameter: start_time or end_time"))
		return
	}

	startNS, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "start_time and end_time must be integers (UTC nanoseconds)"))
		return
	}
	endNS, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "start_time and end_time must be integers (UTC nanoseconds)"))
		return
	}

	result, err := h.db.GetCameraRollup(c.Request.Context(), camera, minutes, startNS, endNS)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// File reading

func (h *Handler) readInitFile(camera string) ([]byte, error) {
	path := filepath.Join(h.dataDir, camera, fmt.Sprintf("init-stream%d.m4s", videoStreamIndex))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, NewAPIError(http.StatusNotFound, fmt.Sprintf("init segment not found: camera=%s", camera))
		}
		return nil, fmt.Errorf("read init file: %w", err)
	}
	return data, nil
}

func (h *Handler) readChunkFile(ctx context.Context, camera string, chunkValue int64) ([]byte, error) {
	prefix := h.resolvePrefix(ctx, camera)

	// chunkValue is epoch milliseconds
	dt := time.UnixMilli(chunkValue).UTC()
	dateDir := dt.Format("20060102")
	filename := fmt.Sprintf("%s%d-%d.m4s", prefix, videoStreamIndex, chunkValue)

	// Try with date folder first
	path := filepath.Join(h.dataDir, camera, dateDir, filename)
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	// Fallback: without date folder
	path = filepath.Join(h.dataDir, camera, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, NewAPIError(http.StatusNotFound, fmt.Sprintf("chunk not found: camera=%s", camera))
		}
		return nil, fmt.Errorf("read chunk file: %w", err)
	}
	return data, nil
}

func (h *Handler) resolvePrefix(ctx context.Context, camera string) string {
	h.mu.RLock()
	if prefix, ok := h.prefixCache[camera]; ok {
		h.mu.RUnlock()
		return prefix
	}
	h.mu.RUnlock()

	prefix, _ := h.db.GetCameraPrefix(ctx, camera)

	h.mu.Lock()
	h.prefixCache[camera] = prefix
	h.mu.Unlock()

	return prefix
}
