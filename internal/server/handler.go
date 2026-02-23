package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"neo-blackbox/internal/db"
	"neo-blackbox/internal/ffmpeg"
	"neo-blackbox/internal/logger"
	"neo-blackbox/internal/watcher"

	"github.com/gin-gonic/gin"
)

const videoStreamIndex = 0

var defaultSensorNames = []string{
	"sensor-1", "sensor-2", "sensor-3", "sensor-4", "sensor-5",
	"sensor-6", "sensor-7", "sensor-8", "sensor-9", "sensor-10",
}

var defaultSensorLabels = map[string]string{
	"sensor-1":  "Sensor 1",
	"sensor-2":  "Sensor 2",
	"sensor-3":  "Sensor 3",
	"sensor-4":  "Sensor 4",
	"sensor-5":  "Sensor 5",
	"sensor-6":  "Sensor 6",
	"sensor-7":  "Sensor 7",
	"sensor-8":  "Sensor 8",
	"sensor-9":  "Sensor 9",
	"sensor-10": "Sensor 10",
}

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

// cameraProcess tracks a running ffmpeg process for a camera.
type cameraProcess struct {
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	startedAt time.Time
}

// Handler handles API requests.
type Handler struct {
	machbase             *db.Machbase
	watcher              Watcher // watcher interface for dynamic watch management
	dataDir              string
	logDir               string // log directory (ffmpeg logs etc.)
	mvsDir               string
	cameraDir            string
	ffmpegBinary         string
	ffRunner             *ffmpeg.FFmpegRunner
	mediamtxHost         string // MediaMTX 서버 호스트
	mediamtxPort         int    // MediaMTX 서버 포트
	prefixCache          map[string]string
	fpsCache             map[string]*int
	cacheMu              sync.RWMutex
	processes            map[string]*cameraProcess
	processMu            sync.Mutex
	edgeState            map[string]bool // EDGE_ONLY 이전 상태: "camera_id.rule_id" → prev_result
	edgeMu               sync.Mutex
	cameraConfigs        map[string]*CameraCreateRequest // camera_id → full camera config 캐시
	configMu             sync.RWMutex
	detectObjects        []string // 감지 가능한 객체 목록 캐시
	detectObjectMu       sync.RWMutex
	lastEventQueryTime   int64 // 마지막 이벤트 조회 시간 (nanoseconds)
	lastEventQueryTimeMu sync.Mutex
}

// Watcher interface for adding/removing file system watches dynamically
type Watcher interface {
	AddWatch(ctx context.Context, rule watcher.WatcherRule) error
	RemoveWatch(ctx context.Context, cameraID string) error
}

// NewHandler creates a new Handler.
func NewHandler(machbase *db.Machbase, watcher Watcher, ffRunner *ffmpeg.FFmpegRunner, dataDir, logDir, mvsDir, cameraDir, ffmpegBinary string, mediamtxHost string, mediamtxPort int) *Handler {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logDir == "" {
		logDir = "/var/log/blackbox"
	}
	h := &Handler{
		machbase:      machbase,
		watcher:       watcher,
		ffRunner:      ffRunner,
		dataDir:       dataDir,
		logDir:        logDir,
		mvsDir:        mvsDir,
		cameraDir:     cameraDir,
		ffmpegBinary:  ffmpegBinary,
		mediamtxHost:  mediamtxHost,
		mediamtxPort:  mediamtxPort,
		prefixCache:   make(map[string]string),
		fpsCache:      make(map[string]*int),
		processes:     make(map[string]*cameraProcess),
		edgeState:     make(map[string]bool),
		cameraConfigs: make(map[string]*CameraCreateRequest),
	}
	h.loadAllCameraConfigs()

	// COCO 80 classes (hardcoded)
	h.detectObjects = []string{
		"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat",
		"traffic light", "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat",
		"dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe", "backpack",
		"umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard", "sports ball",
		"kite", "baseball bat", "baseball glove", "skateboard", "surfboard", "tennis racket",
		"bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
		"sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair",
		"couch", "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse",
		"remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink", "refrigerator",
		"book", "clock", "vase", "scissors", "teddy bear", "hair drier", "toothbrush",
	}

	return h
}

// loadAllCameraConfigs scans cameraDir and pre-loads all camera configs into cache.
func (h *Handler) loadAllCameraConfigs() {
	if h.cameraDir == "" {
		return
	}
	entries, err := os.ReadDir(h.cameraDir)
	if err != nil {
		logger.GetLogger().Warnf("[camera_configs] cameraDir not found, skip preload: %v", err)
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		cameraID := strings.TrimSuffix(entry.Name(), ".json")
		config, err := h.loadCameraConfigFromFile(cameraID)
		if err != nil {
			logger.GetLogger().Errorf("[camera_configs] failed to load %s: %v", cameraID, err)
			continue
		}
		h.cameraConfigs[cameraID] = config
		if len(config.EventRule) > 0 {
			count += len(config.EventRule)
		}
	}
	logger.GetLogger().Infof("[camera_configs] preloaded %d rules from %d cameras", count, len(h.cameraConfigs))
}

// loadCameraConfigFromFile reads camera config from file. (caller holds lock or no lock needed)
func (h *Handler) loadCameraConfigFromFile(cameraID string) (*CameraCreateRequest, error) {
	path := filepath.Join(h.cameraDir, cameraID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config CameraCreateRequest
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// getCameraConfig returns cached camera config. Loads from file on cache miss.
func (h *Handler) getCameraConfig(cameraID string) *CameraCreateRequest {
	h.configMu.RLock()
	config, ok := h.cameraConfigs[cameraID]
	h.configMu.RUnlock()
	if ok {
		return config
	}

	// cache miss → load from file
	loaded, err := h.loadCameraConfigFromFile(cameraID)
	if err != nil {
		return nil
	}
	h.configMu.Lock()
	h.cameraConfigs[cameraID] = loaded
	h.configMu.Unlock()
	return loaded
}

// refreshCameraConfigCache reloads camera config for a specific camera from file.
func (h *Handler) refreshCameraConfigCache(cameraID string) {
	config, err := h.loadCameraConfigFromFile(cameraID)
	if err != nil {
		logger.GetLogger().Errorf("[camera_configs] failed to refresh %s: %v", cameraID, err)
		return
	}
	h.configMu.Lock()
	h.cameraConfigs[cameraID] = config
	h.configMu.Unlock()
}

// removeCameraConfigCache removes camera config cache for a camera.
func (h *Handler) removeCameraConfigCache(cameraID string) {
	h.configMu.Lock()
	delete(h.cameraConfigs, cameraID)
	h.configMu.Unlock()
}

// Shutdown stops all running ffmpeg processes.
func (h *Handler) Shutdown() {
	h.processMu.Lock()
	procs := make(map[string]*cameraProcess, len(h.processes))
	for k, v := range h.processes {
		procs[k] = v
	}
	h.processMu.Unlock()

	for id, proc := range procs {
		logger.GetLogger().Infof("[camera:%s] shutting down ffmpeg (PID: %d)", id, proc.cmd.Process.Pid)
		proc.cancel()
	}
}

// getDetectObjects returns the hardcoded COCO 80 class list.
func (h *Handler) getDetectObjects() []string {
	return h.detectObjects
}

// errorResponse sends a standardized error response.
func errorResponse(c *gin.Context, tick time.Time, status int, reason string) {
	c.JSON(status, Response{
		Success: false,
		Reason:  reason,
		Elapse:  time.Since(tick).String(),
		Data:    nil,
	})
}

// successResponse sends a standardized success response.
func successResponse(c *gin.Context, tick time.Time, data any) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Reason:  "success",
		Elapse:  time.Since(tick).String(),
		Data:    data,
	})
}

// sanitizeTag validates and sanitizes a tag value.
func sanitizeTag(value string) (string, error) {
	if value == "" {
		return "", ErrEmptyTag
	}
	if !tagPattern.MatchString(value) {
		return "", ErrIllegalTagCharacters
	}
	return value, nil
}

// parseTimeToken parses a time token string.
// Supports: Unix nanoseconds (integer), ISO date strings, or "now".
func parseTimeToken(raw string) (time.Time, error) {
	candidate := strings.TrimSpace(raw)
	if strings.ToLower(candidate) == "now" {
		return time.Now(), nil
	}

	// Try parsing as Unix nanoseconds (integer)
	if nsInt, err := strconv.ParseInt(candidate, 10, 64); err == nil {
		if nsInt > 0 {
			sec := nsInt / 1_000_000_000
			nsec := nsInt % 1_000_000_000
			return time.Unix(sec, nsec), nil
		}
	}

	// Fall back to ISO date string parsing
	normalized := strings.ReplaceAll(candidate, "T", " ")
	normalized = strings.TrimSuffix(normalized, "Z")

	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, normalized, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, ErrInvalidTimeFormat
}

// formatTime formats a time value for response.
func formatTime(t time.Time) string {
	s := t.UTC().Format("2006-01-02T15:04:05.000000")
	if idx := strings.Index(s, "."); idx != -1 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s + "Z"
}

// resolvePrefix resolves the chunk prefix for a camera.
func (h *Handler) resolvePrefix(c *gin.Context, camera string) string {
	h.cacheMu.RLock()
	if prefix, ok := h.prefixCache[camera]; ok {
		h.cacheMu.RUnlock()
		return prefix
	}
	h.cacheMu.RUnlock()

	// Get prefix from camera config file instead of DB metadata
	config := h.getCameraConfig(camera)
	if config == nil {
		h.cacheMu.Lock()
		h.prefixCache[camera] = "chunk-stream"
		h.fpsCache[camera] = nil
		h.cacheMu.Unlock()
		return "chunk-stream"
	}

	// Use default prefix for now
	// TODO: add prefix field to config if custom prefix is needed
	prefix := "chunk-stream"

	h.cacheMu.Lock()
	h.prefixCache[camera] = prefix
	h.fpsCache[camera] = nil // FPS not stored in config yet
	h.cacheMu.Unlock()

	return prefix
}

// getCameraFPS gets the FPS for a camera.
func (h *Handler) getCameraFPS(c *gin.Context, camera string) *int {
	h.cacheMu.RLock()
	if fps, ok := h.fpsCache[camera]; ok {
		h.cacheMu.RUnlock()
		return fps
	}
	h.cacheMu.RUnlock()

	h.resolvePrefix(c, camera)

	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	return h.fpsCache[camera]
}

// resolveArchiveDir returns the archive directory for a camera.
// Uses camera config's archive_dir if set (absolute path), otherwise defaults to {dataDir}/{camera}/out.
func (h *Handler) resolveArchiveDir(cameraID string) string {
	config := h.getCameraConfig(cameraID)
	if config != nil && config.ArchiveDir != "" && filepath.IsAbs(config.ArchiveDir) {
		return config.ArchiveDir
	}
	return filepath.Join(h.dataDir, cameraID, "out")
}

// initPath returns the path to the init segment.
func (h *Handler) initPath(camera string) string {
	return filepath.Join(h.resolveArchiveDir(camera), "init-stream0.m4s")
}

// chunkPath returns the path to a chunk segment.
func (h *Handler) chunkPath(c *gin.Context, camera string, chunkNumber int64) string {
	prefix := h.resolvePrefix(c, camera)
	t := time.UnixMilli(chunkNumber).UTC()
	dateDir := t.Format("20060102")
	filename := prefix + "0-" + time.UnixMilli(chunkNumber).UTC().Format("20060202150405") + ".m4s"
	return filepath.Join(h.dataDir, camera, "out", dateDir, filename)
}

// readChunkFile reads a chunk file.
func (h *Handler) readChunkFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// sensorSortKey returns a sort key for sensor IDs.
func sensorSortKey(sensorID string) (int, int, string) {
	re := regexp.MustCompile(`^sensor-(\d+)$`)
	if matches := re.FindStringSubmatch(sensorID); len(matches) == 2 {
		var num int
		_, _ = parseIntValue(matches[1], &num)
		return 0, num, ""
	}
	return 1, 0, sensorID
}

// sortSensorIDs sorts sensor IDs.
func sortSensorIDs(sensorIDs []string) {
	sort.Slice(sensorIDs, func(i, j int) bool {
		a1, a2, a3 := sensorSortKey(sensorIDs[i])
		b1, b2, b3 := sensorSortKey(sensorIDs[j])
		if a1 != b1 {
			return a1 < b1
		}
		if a2 != b2 {
			return a2 < b2
		}
		return a3 < b3
	})
}

func parseIntValue(s string, out *int) (bool, error) {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
	}
	var val int
	for _, c := range s {
		val = val*10 + int(c-'0')
	}
	*out = val
	return true, nil
}

// sensorKeyFromTag extracts sensor key from tag.
func sensorKeyFromTag(camera, tag string) string {
	if tag == "" {
		return ""
	}
	prefix1 := camera + ":"
	prefix2 := camera + "."
	if strings.HasPrefix(tag, prefix1) {
		return tag[len(prefix1):]
	}
	if strings.HasPrefix(tag, prefix2) {
		return tag[len(prefix2):]
	}
	return tag
}

// matchSensorID matches a tag name to a sensor ID.
func matchSensorID(tagName string, sensorIDs []string) string {
	for _, sensorID := range sensorIDs {
		if tagName == sensorID {
			return sensorID
		}
		if strings.HasSuffix(tagName, ":"+sensorID) || strings.HasSuffix(tagName, "."+sensorID) {
			return sensorID
		}
	}
	return ""
}

// uniqueStrings returns unique strings from a slice.
func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// Error types
type ApiError struct {
	Status  int
	Message string
}

func (e ApiError) Error() string {
	return e.Message
}

func NewApiError(status int, message string) ApiError {
	return ApiError{Status: status, Message: message}
}

var (
	ErrEmptyTag             = NewApiError(http.StatusBadRequest, "Empty tag value")
	ErrIllegalTagCharacters = NewApiError(http.StatusBadRequest, "Illegal characters in tag")
	ErrInvalidTimeFormat    = NewApiError(http.StatusBadRequest, "Invalid time format")
)
