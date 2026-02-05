package server

import (
	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"blackbox-backend/internal/dsl"
	"blackbox-backend/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type EventRule struct {
	ID         string `json:"rule_id"`
	Name       string `json:"name"`
	Expression string `json:"expression_text"`
	RecordMode string `json:"record_mode"`
	Enabled    bool   `json:"enabled"`
}

type MvsCameraCreateRequest struct {
	CameraID      string   `json:"camera_id"`                         // cam{id}_{model_id}_{time} (자동 생성 가능)
	CameraURL     string   `json:"camera_url" binding:"required"`     // rtsp URL
	ModelID       int      `json:"model_id"`                          // 기본 모델 0
	DetectObjects []string `json:"detect_objects" binding:"required"` // ex) ["person", "car", "truck", "bus"]
}

// 이 구조체는 테이블이 아닌 파일로 저장이됨, JSON형식
type CameraCreateRequest struct {
	Enabled bool
	Table   string `json:"table" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Desc    string `json:"desc"`

	RtspURL   string `json:"rtsp_url"` // binding:"url" 어떤 url
	WebRTCURL string `json:"webrtc_url"`
	MediaURL  string `json:"media_url"` // 미디어 서버 URL

	ModelID       int      `json:"model_id"`
	DetectObjects []string `json:"detect_objects"` // ex) ["person", "car", "truck", "bus"]
	SaveObjects   bool     `json:"save_objects"`   // {camera}_log 테이블에 데이터 저장 여부

	FFmpegOptions []ReqKV `json:"ffmpeg_options"` // 프론트에 전달 필요
	OutputDir     string  `json:"output_dir"`     // ffmpeg 출력 디렉토리
	OutputName    string  `json:"output_name"`    // ffmpeg 출력 파일명 (e.g., manifest.mpd)

	EventRule []EventRule // request에서는 안 받지만, 별도로 eventRule을 받는 API가 있고 CameraCreateRequest의 구조체는 파일에 json으로 저장됨
}

type ReqKV struct {
	K string  `json:"k" binding:"required"`
	V *string `json:"v"`
}

// CreateCamera handles POST /api/camera.
// 1. Saves camera config as JSON file in cameraDir
// 2. Creates 3 tables: {name}, {name}_event, {name}_log
// 3. Saves MVS config file in mvsDir (for detection)
func (h *Handler) CreateCamera(c *gin.Context) {
	tick := time.Now()

	var req CameraCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "bad request parameter")
		return
	}

	// Validate name (used as table name)
	if req.Name == "" {
		errorResponse(c, tick, http.StatusBadRequest, "name is required")
		return
	}

	// 1. Save camera config as JSON file
	if err := os.MkdirAll(h.cameraDir, 0755); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to create camera directory")
		return
	}

	req.Enabled = true
	cameraJSON, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal camera config")
		return
	}

	cameraPath := filepath.Join(h.cameraDir, req.Name+".json")
	if err := os.WriteFile(cameraPath, cameraJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write camera config file")
		return
	}

	// 2. Create 3 tables: {name}, {name}_event, {name}_log
	if err := h.machbase.CreateCameraTables(c.Request.Context(), req.Name); err != nil {
		// Rollback: delete the config file
		_ = os.Remove(cameraPath)
		errorResponse(c, tick, http.StatusInternalServerError, fmt.Sprintf("failed to create camera tables: %v", err))
		return
	}

	// 3. Save MVS config file (for detection program)
	if err := os.MkdirAll(h.mvsDir, 0755); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to create mvs directory")
		return
	}

	// Build MVS config from request fields
	mvs := MvsCameraCreateRequest{
		CameraID:      req.Name,
		CameraURL:     req.RtspURL,
		ModelID:       req.ModelID,
		DetectObjects: req.DetectObjects,
	}

	mvsJSON, err := json.MarshalIndent(mvs, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal mvs data")
		return
	}

	mvsFileName := fmt.Sprintf("%s_%d_%d.mvs", mvs.CameraID, mvs.ModelID, time.Now().Unix())
	mvsPath := filepath.Join(h.mvsDir, mvsFileName)
	if err := os.WriteFile(mvsPath, mvsJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write mvs file")
		return
	}

	// 4. Event rules 캐시 초기화
	h.refreshCameraConfigCache(req.Name)

	c.JSON(http.StatusOK, Response{
		Success: true,
		Reason:  "success",
		Elapse:  time.Since(tick).String(),
		Data: CreateCameraResponse{
			Name:       req.Name,
			ConfigPath: cameraPath,
			Tables: []string{
				req.Name,
				req.Name + "_event",
				req.Name + "_log",
			},
			MvsPath: mvsPath,
		},
	})
}

type CameraInfoResponse struct {
	Table string `json:"table"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`

	RtspURL   string `json:"rtsp_url"`
	WebRTCUrl string `json:"webrtc_url"`
	MediaURL  string `json:"media_url"`

	EventRule     string `json:"event_rule"`
	DetectObjects string `json:"detect_objects"`
	FFmpegOption  string `json:"ffmpeg_option"`
}

// ============================================================
// MVS (Machine Vision System) Camera
// ============================================================

// type MvsCameraCreateRequest struct {
// 	CameraID      string   `json:"camera_id"`                         // cam{id}_{model_id}_{time} (자동 생성 가능)
// 	CameraURL     string   `json:"camera_url" binding:"required"`     // rtsp URL
// 	ModelID       int      `json:"model_id"`                          // 기본 모델 0
// 	DetectObjects []string `json:"detect_objects" binding:"required"` // ex) ["person", "car", "truck", "bus"]
// }

// CreateMvsCamera handles POST /api/mvs/camera.
// MVS 파일 저장용 - camera_id, camera_url, model_id, detect_objects만 저장.
func (h *Handler) CreateMvsCamera(c *gin.Context) {
	tick := time.Now()

	var req MvsCameraCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Reason:  "bad request parameter",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	// camera_id 자동 생성: cam{id}_{model_id}_{unix_time}
	if req.CameraID == "" {
		req.CameraID = fmt.Sprintf("cam%d_%d_%d", time.Now().UnixNano()%100000, req.ModelID, time.Now().Unix())
	}

	// .mvs 파일로 저장
	mvsData := map[string]any{
		"camera_id":      req.CameraID,
		"camera_url":     req.CameraURL,
		"model_id":       req.ModelID,
		"detect_objects": req.DetectObjects,
	}

	mvsJSON, err := json.MarshalIndent(mvsData, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Reason:  "failed to marshal mvs data",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	if err := os.MkdirAll(h.mvsDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Reason:  "failed to create mvs directory",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	mvsFileName := fmt.Sprintf("%s_%d_%d.mvs", req.CameraID, req.ModelID, time.Now().Unix())
	mvsPath := filepath.Join(h.mvsDir, mvsFileName)
	if err := os.WriteFile(mvsPath, mvsJSON, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Reason:  "failed to write mvs file",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Reason:  "success",
		Elapse:  time.Since(tick).String(),
		Data: CreateMvsCameraResponse{
			CameraID: req.CameraID,
			MvsPath:  mvsPath,
		},
	})
}

// ============================================================
// MVS Event (외부 프로그램 → detection 결과 수신)
// ============================================================

type AIResultRequest struct {
	CameraID     string             `json:"camera_id" binding:"required"`
	ModelID      int64              `json:"model_id"`                      // 기본값 0
	Timestamp    int64              `json:"timestamp"`                     // Unix timestamp in milliseconds
	Detections   map[string]float64 `json:"detections" binding:"required"` // {"person": 3, "car": 5, ...}
	TotalObjects int                `json:"total_objects"`
}

// CreateMvsEvent handles POST /api/ai/results.
// 외부 AI 프로그램에서 detection 결과를 수신.
// 1) OR_LOG: {camera}_log 테이블에 ident별 count 저장
// 2) EventLog: 카메라 설정의 event_rule DSL 평가 → {camera}_event 테이블에 결과 저장
func (h *Handler) UploadAIResult(c *gin.Context) {
	tick := time.Now()

	var req AIResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.GetLogger().Errorf("upload AI result: %v", err)
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Reason:  "bad request parameter",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	// timestamp: milliseconds → nanoseconds
	if req.Timestamp <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Reason:  "invalid timestamp: must be positive milliseconds",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}
	tsNano := req.Timestamp * 1000000 // milliseconds to nanoseconds

	config := h.getCameraConfig(req.CameraID)
	if config == nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Reason:  "camera config not found",
			Elapse:  time.Since(tick).String(),
			Data:    nil,
		})
		return
	}

	// 1) OR_LOG: SaveObjects가 true일 때만 detections → {camera}_log 테이블에 저장
	if config.SaveObjects {
		logs := make([]db.CameraLogRow, 0, len(req.Detections))
		for ident, value := range req.Detections {
			logs = append(logs, db.CameraLogRow{
				Name:     req.CameraID + "." + ident,
				Time:     tsNano,
				Value:    value,
				ModelID:  req.ModelID,
				CameraID: req.CameraID,
				Ident:    ident,
			})
		}

		if err := h.machbase.InsertCameraLogs(c.Request.Context(), req.CameraID+"_log", logs); err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Success: false,
				Reason:  "failed to insert camera logs",
				Elapse:  time.Since(tick).String(),
				Data:    nil,
			})
			return
		}
	}

	// 2) EventLog: 캐시된 event rules로 DSL 평가 → {camera}_event 저장
	_ = h.evaluateEventRules(c.Request.Context(), req.CameraID, tsNano, req.Detections, config.EventRule)

	c.JSON(http.StatusOK, Response{
		Success: true,
		Reason:  "success",
		Elapse:  time.Since(tick).String(),
		Data:    nil,
		// Data: CreateMvsEventResponse{
		// CameraID: req.CameraID,
		// },
	})
}

// evaluateEventRules evaluates all enabled event rules against detection counts.
// Returns the number of event rows inserted.
func (h *Handler) evaluateEventRules(ctx context.Context, cameraID string, tsNano int64, counts map[string]float64, rules []EventRule) int {
	if len(rules) == 0 {
		return 0
	}

	eventTable := cameraID + "_event"
	var events []db.CameraEventRow

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		result, err := dsl.Evaluate(rule.Expression, counts)
		if err != nil {
			logger.GetLogger().Errorf("[camera:%s][rule:%s] DSL parse error: %v", cameraID, rule.ID, err)
			continue
		}

		// used_counts_snapshot 생성
		snapshot := make(map[string]any, len(counts)+1)
		for k, v := range counts {
			snapshot[k] = v
		}
		if result.Error != "" {
			snapshot["_error"] = result.Error
		}
		snapshotJSON, _ := json.Marshal(snapshot)

		stateKey := cameraID + "." + rule.ID
		var valueCode float64
		var shouldRecord bool

		if result.Error != "" {
			// ERROR(-1): 항상 기록, EDGE_ONLY 상태 변경 안 함
			valueCode = -1
			shouldRecord = true
		} else {
			switch rule.RecordMode {
			case "ALL_MATCHES":
				if result.Value {
					valueCode = 2 // MATCH
					shouldRecord = true
				}
			case "EDGE_ONLY":
				h.edgeMu.Lock()
				prev := h.edgeState[stateKey]
				if result.Value && !prev {
					valueCode = 1 // TRIGGER (false → true)
					shouldRecord = true
					h.edgeState[stateKey] = true
				} else if !result.Value && prev {
					valueCode = 0 // RESOLVE (true → false)
					shouldRecord = true
					h.edgeState[stateKey] = false
				}
				h.edgeMu.Unlock()
			}
		}

		if shouldRecord {
			events = append(events, db.CameraEventRow{
				Name:               stateKey,
				Time:               tsNano,
				Value:              valueCode,
				ExpressionText:     rule.Expression,
				UsedCountsSnapshot: string(snapshotJSON),
				CameraID:           cameraID,
				RuleID:             rule.ID,
			})
		}
	}

	if len(events) > 0 {
		if err := h.machbase.InsertCameraEvents(ctx, eventTable, events); err != nil {
			logger.GetLogger().Errorf("[camera:%s] failed to insert events: %v", cameraID, err)
			return 0
		}
	}

	return len(events)
}

// GetCamera handles GET /api/camera/:id.
// camera_dir 안의 {id}.json 파일을 읽어서 그대로 리턴.
func (h *Handler) GetCamera(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	cameraPath := filepath.Join(h.cameraDir, id+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", id))
			return
		}
		errorResponse(c, tick, http.StatusInternalServerError, "failed to read camera config")
		return
	}

	var camera CameraCreateRequest
	if err := json.Unmarshal(data, &camera); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to parse camera config")
		return
	}

	successResponse(c, tick, camera)
}

// UpdateCamera handles POST /api/camera/:id.
// camera_dir 안의 {id}.json 파일 내용을 수정하고 저장.
func (h *Handler) UpdateCamera(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	cameraPath := filepath.Join(h.cameraDir, id+".json")

	// 기존 파일 존재 확인
	if _, err := os.Stat(cameraPath); os.IsNotExist(err) {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", id))
		return
	}

	var req CameraCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "bad request parameter")
		return
	}

	// id는 URL에서 가져오므로 Name/Table 고정
	req.Name = id
	req.Table = id

	cameraJSON, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal camera config")
		return
	}

	if err := os.WriteFile(cameraPath, cameraJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write camera config file")
		return
	}

	// MVS 파일도 새 시간으로 갱신
	mvs := MvsCameraCreateRequest{
		CameraID:      id,
		CameraURL:     req.RtspURL,
		ModelID:       req.ModelID,
		DetectObjects: req.DetectObjects,
	}

	mvsJSON, err := json.MarshalIndent(mvs, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal mvs data")
		return
	}

	mvsFileName := fmt.Sprintf("%s_%d_%d.mvs", id, req.ModelID, time.Now().Unix())
	mvsPath := filepath.Join(h.mvsDir, mvsFileName)
	if err := os.WriteFile(mvsPath, mvsJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write mvs file")
		return
	}

	// Event rules 캐시 갱신
	h.refreshCameraConfigCache(id)

	successResponse(c, tick, CreateCameraResponse{
		Name:       id,
		ConfigPath: cameraPath,
		MvsPath:    mvsPath,
	})
}

// DeleteCamera handles DELETE /api/camera/:id.
// camera_dir 안의 {id}.json 파일 삭제.
func (h *Handler) DeleteCamera(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	cameraPath := filepath.Join(h.cameraDir, id+".json")

	if _, err := os.Stat(cameraPath); os.IsNotExist(err) {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", id))
		return
	}

	if err := os.Remove(cameraPath); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to delete camera config file")
		return
	}

	// Event rules 캐시 제거
	h.removeCameraConfigCache(id)

	successResponse(c, tick, map[string]string{
		"name": id,
	})
}

// TestCameraConnection handles POST /api/camera/test.
// Tests RTSP URL connection.
func (h *Handler) TestCameraConnection(c *gin.Context) {
	// TODO: implement
	c.JSON(http.StatusNotImplemented, ErrorResponse{Error: "not implemented"})
}

// EnableCamera handles POST /api/camera/:id/enable.
// 카메라 설정파일을 읽어서 ffmpeg 프로세스를 시작.
func (h *Handler) EnableCamera(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	if h.ffmpegBinary == "" {
		errorResponse(c, tick, http.StatusInternalServerError, "ffmpeg binary not configured")
		return
	}

	// 이미 실행 중인지 확인
	h.processMu.Lock()
	if _, running := h.processes[id]; running {
		h.processMu.Unlock()
		errorResponse(c, tick, http.StatusConflict, fmt.Sprintf("camera '%s' is already running", id))
		return
	}
	h.processMu.Unlock()

	// 카메라 설정 파일 읽기
	cameraPath := filepath.Join(h.cameraDir, id+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", id))
			return
		}
		errorResponse(c, tick, http.StatusInternalServerError, "failed to read camera config")
		return
	}

	var cam CameraCreateRequest
	if err := json.Unmarshal(data, &cam); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to parse camera config")
		return
	}

	if cam.RtspURL == "" {
		errorResponse(c, tick, http.StatusBadRequest, "camera has no rtsp_url configured")
		return
	}

	// ffmpeg 인자 빌드
	args := buildFFmpegArgs(cam)

	// output_dir 준비
	outputDir := cam.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(h.dataDir, id, "in")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to create output directory")
		return
	}

	// ffmpeg 프로세스 시작
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, h.ffmpegBinary, args...)
	cmd.Dir = outputDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.GetLogger().Infof("[camera:%s] ffmpeg start: %s %s", id, h.ffmpegBinary, strings.Join(args, " "))

	if err := cmd.Start(); err != nil {
		cancel()
		errorResponse(c, tick, http.StatusInternalServerError, fmt.Sprintf("failed to start ffmpeg: %v", err))
		return
	}

	h.processMu.Lock()
	h.processes[id] = &cameraProcess{cmd: cmd, cancel: cancel, startedAt: time.Now()}
	h.processMu.Unlock()

	// watcher에 rule 추가 (ffmpeg가 생성하는 파일을 감시)
	targetDir := filepath.Join(h.dataDir, id, "out")
	rule := config.WatcherRule{
		CameraID:  id,
		SourceDir: outputDir,
		TargetDir: targetDir,
		Ext:       ".m4s",
	}

	if err := h.watcher.AddWatch(c.Request.Context(), rule); err != nil {
		// watcher 추가 실패 시 ffmpeg 중지 (rollback)
		logger.GetLogger().Errorf("[camera:%s] failed to add watcher, stopping ffmpeg: %v", id, err)
		cancel()
		h.processMu.Lock()
		delete(h.processes, id)
		h.processMu.Unlock()
		errorResponse(c, tick, http.StatusInternalServerError, fmt.Sprintf("failed to add watcher: %v", err))
		return
	}

	// 프로세스 종료 감시 (비동기)
	go func() {
		err := cmd.Wait()
		h.processMu.Lock()
		delete(h.processes, id)
		h.processMu.Unlock()

		// ffmpeg 종료 시 watcher도 제거
		if err := h.watcher.RemoveWatch(context.Background(), id); err != nil {
			logger.GetLogger().Errorf("[camera:%s] failed to remove watcher: %v", id, err)
		}

		if err != nil {
			logger.GetLogger().Warnf("[camera:%s] ffmpeg exited: %v", id, err)
		} else {
			logger.GetLogger().Infof("[camera:%s] ffmpeg exited normally", id)
		}
	}()

	successResponse(c, tick, map[string]any{
		"name":   id,
		"pid":    cmd.Process.Pid,
		"status": "running",
	})
}

// DisableCamera handles POST /api/camera/:id/disable.
// 실행 중인 ffmpeg 프로세스를 중지.
func (h *Handler) DisableCamera(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	h.processMu.Lock()
	proc, running := h.processes[id]
	if !running {
		h.processMu.Unlock()
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' is not running", id))
		return
	}
	delete(h.processes, id)
	h.processMu.Unlock()

	// ffmpeg 중지
	proc.cancel()

	// watcher 제거 (ffmpeg 종료 go routine에서도 제거하지만, 명시적으로 제거)
	if err := h.watcher.RemoveWatch(c.Request.Context(), id); err != nil {
		logger.GetLogger().Warnf("[camera:%s] failed to remove watcher: %v", id, err)
		// 에러가 발생해도 계속 진행 (이미 제거되었을 수 있음)
	}

	successResponse(c, tick, map[string]string{
		"name":   id,
		"status": "stopped",
	})
}

// buildFFmpegArgs builds ffmpeg command args from camera config.
// input options → -i rtsp_url → output options → output_name
func buildFFmpegArgs(cam CameraCreateRequest) []string {
	var inputArgs, outputArgs []string

	for _, opt := range cam.FFmpegOptions {
		key := opt.K
		flag := key
		if !strings.HasPrefix(flag, "-") {
			flag = "-" + flag
		}

		if isOutputOption(key) {
			outputArgs = append(outputArgs, flag)
			if opt.V != nil {
				outputArgs = append(outputArgs, *opt.V)
			}
		} else {
			inputArgs = append(inputArgs, flag)
			if opt.V != nil {
				inputArgs = append(inputArgs, *opt.V)
			}
		}
	}

	args := make([]string, 0, len(inputArgs)+len(outputArgs)+4)
	args = append(args, inputArgs...)
	args = append(args, "-i", cam.RtspURL)
	args = append(args, outputArgs...)

	outputName := cam.OutputName
	if outputName == "" {
		outputName = "manifest.mpd"
	}
	args = append(args, outputName)

	return args
}

// isOutputOption returns true if the key is an ffmpeg output option (goes after -i).
func isOutputOption(key string) bool {
	// codec options
	if strings.HasPrefix(key, "c:") || strings.HasPrefix(key, "codec") ||
		key == "vcodec" || key == "acodec" {
		return true
	}
	// format & segment options
	switch key {
	case "f", "format",
		"seg_duration", "segment_time", "segment_format",
		"use_template", "use_timeline",
		"window_size", "extra_window_size", "min_seg_duration",
		"hls_time", "hls_list_size", "hls_segment_filename", "hls_flags",
		"movflags", "frag_type",
		"b", "b:v", "b:a", "preset", "crf",
		"r", "s", "an", "vn", "map",
		"copyts":
		return true
	}
	return false
}

// GetCameraStatus handles GET /api/camera/:id/status.
// 개별 카메라의 설정 존재 여부, ffmpeg 프로세스 실행 상태를 리턴.
func (h *Handler) GetCameraStatus(c *gin.Context) {
	tick := time.Now()
	id := c.Param("id")

	// 설정 파일 존재 확인
	cameraPath := filepath.Join(h.cameraDir, id+".json")
	_, err := os.Stat(cameraPath)
	if os.IsNotExist(err) {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", id))
		return
	}

	status := "stopped"
	var pid int
	var startedAt string
	var uptime string

	h.processMu.Lock()
	proc, running := h.processes[id]
	if running {
		status = "running"
		pid = proc.cmd.Process.Pid
		startedAt = proc.startedAt.Format(time.RFC3339)
		uptime = time.Since(proc.startedAt).Truncate(time.Second).String()
	}
	h.processMu.Unlock()

	resp := map[string]any{
		"name":   id,
		"status": status,
	}
	if running {
		resp["pid"] = pid
		resp["started_at"] = startedAt
		resp["uptime"] = uptime
	}

	successResponse(c, tick, resp)
}

// GetCamerasHealth handles GET /api/cameras/health.
// 전체 카메라 헬스 요약: 총 카메라 수, 실행 중, 중지 상태.
func (h *Handler) GetCamerasHealth(c *gin.Context) {
	tick := time.Now()

	entries, err := os.ReadDir(h.cameraDir)
	if err != nil {
		if os.IsNotExist(err) {
			successResponse(c, tick, map[string]any{
				"total":   0,
				"running": 0,
				"stopped": 0,
				"cameras": []any{},
			})
			return
		}
		errorResponse(c, tick, http.StatusInternalServerError, "failed to read camera directory")
		return
	}

	h.processMu.Lock()
	defer h.processMu.Unlock()

	var cameras []map[string]any
	runningCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		cam := map[string]any{
			"name":   name,
			"status": "stopped",
		}

		if proc, ok := h.processes[name]; ok {
			cam["status"] = "running"
			cam["pid"] = proc.cmd.Process.Pid
			cam["started_at"] = proc.startedAt.Format(time.RFC3339)
			cam["uptime"] = time.Since(proc.startedAt).Truncate(time.Second).String()
			runningCount++
		}

		cameras = append(cameras, cam)
	}

	total := len(cameras)
	successResponse(c, tick, map[string]any{
		"total":   total,
		"running": runningCount,
		"stopped": total - runningCount,
		"cameras": cameras,
	})
}
