package server

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func errorResponse(c *gin.Context, tick time.Time, status int, reason string) {
	c.JSON(status, Response{
		Success: false,
		Reason:  reason,
		Elapse:  time.Since(tick).String(),
		Data:    nil,
	})
}

func successResponse(c *gin.Context, tick time.Time, data any) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Reason:  "success",
		Elapse:  time.Since(tick).String(),
		Data:    data,
	})
}

// GetCameras handles GET /api/cameras.
// Returns list of cameras from config files in cameraDir.
func (h *Handler) GetCameras(c *gin.Context) {
	tick := time.Now()

	// Read camera config files from cameraDir
	entries, err := os.ReadDir(h.cameraDir)
	if err != nil {
		if os.IsNotExist(err) {
			successResponse(c, tick, GetCamerasResponse{
				Cameras: []Camera{},
			})
			return
		}
		errorResponse(c, tick, http.StatusInternalServerError, "Failed to read camera directory")
		return
	}

	var cameraList []Camera
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		// Remove .json suffix to get camera name
		cameraName := strings.TrimSuffix(name, ".json")
		cameraList = append(cameraList, Camera{ID: cameraName, Label: cameraName})
	}

	sort.Slice(cameraList, func(i, j int) bool {
		return cameraList[i].ID < cameraList[j].ID
	})

	successResponse(c, tick, GetCamerasResponse{
		Cameras: cameraList,
	})
}

// GetTimeRange handles GET /api/get_time_range.
func (h *Handler) GetTimeRange(c *gin.Context) {
	tick := time.Now()

	var req GetTimeRangeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameter 'tagname'")
		return
	}

	camera, err := sanitizeTag(req.Tagname)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	var start, end *string

	stats, err := h.machbase.BlackboxStatsByTag(ctx, camera)
	if err == nil && stats != nil {
		minStr := formatTime(stats.MinTime)
		maxStr := formatTime(stats.MaxTime)
		start = &minStr
		end = &maxStr
	}

	if start == nil || end == nil {
		bounds, err := h.machbase.BlackboxTimeBounds(ctx, camera)
		if err == nil && bounds != nil {
			if start == nil {
				minStr := formatTime(bounds.MinTime)
				start = &minStr
			}
			if end == nil {
				maxStr := formatTime(bounds.MaxTime)
				end = &maxStr
			}
		}
	}

	if start == nil || end == nil {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("No timeline entries for camera '%s'", camera))
		return
	}

	chunkDuration := 0.0
	interval, err := h.machbase.BlackboxChunkInterval(ctx, camera)
	if err == nil && interval > 0 {
		chunkDuration = interval
	}

	fps := h.getCameraFPS(c, camera)
	if chunkDuration == 0 && fps != nil && *fps > 0 {
		chunkDuration = 1.0 / float64(*fps)
	}

	if chunkDuration == 0 {
		chunkDuration = 5.0
	}

	successResponse(c, tick, GetTimeRangeResponse{
		Camera:               camera,
		Start:                *start,
		End:                  *end,
		ChunkDurationSeconds: chunkDuration,
		FPS:                  fps,
	})
}

// GetChunkInfo handles GET /api/get_chunk_info.
func (h *Handler) GetChunkInfo(c *gin.Context) {
	tick := time.Now()

	var req GetChunkInfoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameter 'tagname' or 'time'")
		return
	}

	camera, err := sanitizeTag(req.Tagname)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, err.Error())
		return
	}

	timestamp, err := parseTimeToken(req.Time)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	record, err := h.machbase.ChunkRecordForTime(ctx, camera, timestamp)
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "Failed to fetch chunk info")
		return
	}

	if record == nil {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("Chunk not found for camera '%s' at time '%s'", camera, req.Time))
		return
	}

	resp := GetChunkInfoResponse{
		Camera: camera,
		Time:   formatTime(record.EntryTime),
		Length: record.Length,
	}
	if record.Value != 0 {
		sign := record.Value
		resp.Sign = &sign
	}

	successResponse(c, tick, resp)
}

// GetChunk handles GET /api/v_get_chunk.
// Note: This returns binary data, not JSON Response format.
func (h *Handler) GetChunk(c *gin.Context) {
	tick := time.Now()

	var req GetChunkRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameter 'tagname'")
		return
	}

	camera, err := sanitizeTag(req.Tagname)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, err.Error())
		return
	}

	timeToken := req.Time
	if timeToken == "" {
		timeToken = "0"
	}

	var chunkData []byte

	if timeToken == "0" || strings.ToLower(timeToken) == "init" {
		path := h.initPath(camera)
		chunkData, err = os.ReadFile(path)
		if err != nil {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("Segment not found for camera '%s'", camera))
			return
		}
	} else {
		timestamp, err := parseTimeToken(timeToken)
		if err != nil {
			errorResponse(c, tick, http.StatusBadRequest, err.Error())
			return
		}

		ctx := c.Request.Context()
		record, err := h.machbase.ChunkRecordForTime(ctx, camera, timestamp)
		if err != nil {
			errorResponse(c, tick, http.StatusInternalServerError, "Failed to fetch chunk info")
			return
		}

		if record == nil {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("Chunk not found for camera '%s' at time '%s'", camera, timeToken))
			return
		}

		path := h.chunkPath(c, camera, record.Value)
		chunkData, err = os.ReadFile(path)
		if err != nil {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("Segment not found for camera '%s'", camera))
			return
		}
	}

	// Binary response - not JSON
	c.Data(http.StatusOK, "application/octet-stream", chunkData)
}

// GetCameraRollup handles GET /api/get_camera_rollup_info.
func (h *Handler) GetCameraRollup(c *gin.Context) {
	tick := time.Now()

	var req GetCameraRollupRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameters")
		return
	}

	camera, err := sanitizeTag(req.Tagname)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, err.Error())
		return
	}

	minutes := req.Minutes
	if minutes <= 0 {
		minutes = 1
	}

	if req.StartTime < 0 || req.EndTime < 0 {
		errorResponse(c, tick, http.StatusBadRequest, "Start and end time must be non-negative")
		return
	}

	if req.StartTime >= req.EndTime {
		errorResponse(c, tick, http.StatusBadRequest, "Parameter 'start_time' must be earlier than 'end_time'")
		return
	}

	ctx := c.Request.Context()
	rows, err := h.machbase.CameraRollup(ctx, camera, minutes, req.StartTime, req.EndTime)
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "Failed to fetch rollup data")
		return
	}

	rollupRows := make([]RollupRow, len(rows))
	for i, row := range rows {
		rollupRows[i] = RollupRow{
			Time: formatTime(row.Time),
		}
		if row.SumLength != 0 {
			sum := row.SumLength
			rollupRows[i].SumLength = &sum
		}
	}

	startDt := utcNanosecondsToTime(req.StartTime)
	endDt := utcNanosecondsToTime(req.EndTime)

	successResponse(c, tick, GetCameraRollupResponse{
		Camera:      camera,
		Minutes:     minutes,
		StartTimeNs: req.StartTime,
		EndTimeNs:   req.EndTime,
		Start:       formatTime(startDt),
		End:         formatTime(endDt),
		Rows:        rollupRows,
	})
}

// utcNanosecondsToTime converts UTC nanoseconds to time.Time.
func utcNanosecondsToTime(ns int64) time.Time {
	sec := ns / 1_000_000_000
	nsec := ns % 1_000_000_000
	return time.Unix(sec, nsec).Local()
}
