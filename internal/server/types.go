package server

// ============================================================
// Common Types
// ============================================================

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ============================================================
// Camera Types
// ============================================================

// Camera represents a camera entry.
type Camera struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// GetCamerasResponse is the response for /api/cameras.
type GetCamerasResponse struct {
	Cameras []Camera `json:"cameras"`
}

// ============================================================
// Time Range Types
// ============================================================

// GetTimeRangeRequest is the request for /api/get_time_range.
type GetTimeRangeRequest struct {
	Tagname string `form:"tagname" binding:"required"`
}

// GetTimeRangeResponse is the response for /api/get_time_range.
type GetTimeRangeResponse struct {
	Camera               string  `json:"camera"`
	Start                string  `json:"start"`
	End                  string  `json:"end"`
	ChunkDurationSeconds float64 `json:"chunk_duration_seconds"`
	FPS                  *int    `json:"fps,omitempty"`
}

// ============================================================
// Chunk Info Types
// ============================================================

// GetChunkInfoRequest is the request for /api/get_chunk_info.
type GetChunkInfoRequest struct {
	Tagname string `form:"tagname" binding:"required"`
	Time    string `form:"time" binding:"required"`
}

// GetChunkInfoResponse is the response for /api/get_chunk_info.
type GetChunkInfoResponse struct {
	Camera string `json:"camera"`
	Time   string `json:"time"`
	Length float64 `json:"length"`
	Sign   *int64 `json:"sign,omitempty"`
}

// ============================================================
// Get Chunk Types
// ============================================================

// GetChunkRequest is the request for /api/v_get_chunk.
type GetChunkRequest struct {
	Tagname string `form:"tagname" binding:"required"`
	Time    string `form:"time"`
}

// ============================================================
// Camera Rollup Types
// ============================================================

// GetCameraRollupRequest is the request for /api/get_camera_rollup_info.
type GetCameraRollupRequest struct {
	Tagname   string `form:"tagname" binding:"required"`
	Minutes   int    `form:"minutes"`
	StartTime int64  `form:"start_time" binding:"required"`
	EndTime   int64  `form:"end_time" binding:"required"`
}

// RollupRow represents a single rollup row.
type RollupRow struct {
	Time      string   `json:"time"`
	SumLength *float64 `json:"sum_length,omitempty"`
}

// GetCameraRollupResponse is the response for /api/get_camera_rollup_info.
type GetCameraRollupResponse struct {
	Camera      string      `json:"camera"`
	Minutes     int         `json:"minutes"`
	StartTimeNs int64       `json:"start_time_ns"`
	EndTimeNs   int64       `json:"end_time_ns"`
	Start       string      `json:"start"`
	End         string      `json:"end"`
	Rows        []RollupRow `json:"rows"`
}

// ============================================================
// Sensor Types
// ============================================================

// Sensor represents a sensor entry.
type Sensor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// GetSensorsRequest is the request for /api/sensors.
type GetSensorsRequest struct {
	Tagname string `form:"tagname" binding:"required"`
}

// GetSensorsResponse is the response for /api/sensors.
type GetSensorsResponse struct {
	Camera  string   `json:"camera"`
	Sensors []Sensor `json:"sensors"`
}

// ============================================================
// Sensor Data Types
// ============================================================

// GetSensorDataRequest is the request for /api/sensor_data.
type GetSensorDataRequest struct {
	Sensors string `form:"sensors" binding:"required"`
	Start   string `form:"start" binding:"required"`
	End     string `form:"end" binding:"required"`
}

// SensorSample represents a sensor data sample.
type SensorSample struct {
	Time   string             `json:"time"`
	Values map[string]float64 `json:"values"`
}

// GetSensorDataResponse is the response for /api/sensor_data.
type GetSensorDataResponse struct {
	Sensors []string       `json:"sensors"`
	Samples []SensorSample `json:"samples"`
}

// ============================================================
// Camera Create/Update Response Types
// ============================================================

// CreateCameraResponse is the response data for POST /api/camera.
type CreateCameraResponse struct {
	CameraID string `json:"camera_id"`
}

// CreateMvsCameraResponse is the response data for POST /api/mvs/camera.
type CreateMvsCameraResponse struct {
	CameraID string `json:"camera_id"`
	MvsPath  string `json:"mvs_path"`
}

// CreateMvsEventResponse is the response data for POST /api/ai/results.
type CreateMvsEventResponse struct {
	CameraID   string `json:"camera_id"`
	LogCount   int    `json:"log_count"`
	EventCount int    `json:"event_count"`
}
