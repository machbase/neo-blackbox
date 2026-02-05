package server

import (
	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func doGet(t *testing.T, c *http.Client, baseURL, path string, q url.Values) (int, []byte) {
	t.Helper()

	u, err := url.Parse(baseURL + path)
	require.NoError(t, err)
	if q != nil {
		q.Set("rowsArray", "true")
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	require.NoError(t, err)

	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, b
}

func setupTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	cfg, err := config.Load("../config/test.yaml")
	require.NoError(t, err)

	machbase, err := db.NewMachbase(cfg.Machbase)
	require.NoError(t, err)

	svr, err := New(cfg.Server, machbase)
	require.NoError(t, err)

	server := httptest.NewServer(svr)
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = 30 * time.Second // Increased for DB operations

	return server, client
}

func TestAPI_GetCameras(t *testing.T) {
	server, client := setupTestServer(t)

	code, body := doGet(t, client, server.URL, "/api/cameras", nil)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func TestAPI_GetTimeRange(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-0")

	code, body := doGet(t, client, server.URL, "/api/get_time_range", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func TestAPI_GetSensors(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-0")

	code, body := doGet(t, client, server.URL, "/api/sensors", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func TestAPI_GetSensorData(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-0")
	q.Set("sensors", "sensor-1")
	q.Set("start", "2025-10-21T19:28:07.1234")
	q.Set("end", "2025-10-21T19:28:09.1234")

	code, body := doGet(t, client, server.URL, "/api/sensor_data", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func TestAPI_GetChunkInfo(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-0")
	q.Set("time", "2025-10-21T19:29:01.628667")

	code, body := doGet(t, client, server.URL, "/api/get_chunk_info", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func TestAPI_GetVideoChunk(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-0")
	q.Set("time", "2025-10-21T19:29:01.628667")

	code, body := doGet(t, client, server.URL, "/api/v_get_chunk", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body length: %d bytes", len(body))
}

func TestAPI_GetCameraRollupInfo(t *testing.T) {
	server, client := setupTestServer(t)

	q := url.Values{}
	q.Set("tagname", "camera-2")
	q.Set("minutes", "1")
	q.Set("start_time", "1761094496000000000")
	q.Set("end_time", "1761097487652444000")

	code, body := doGet(t, client, server.URL, "/api/get_camera_rollup_info", q)
	require.Equal(t, http.StatusOK, code, string(body))

	t.Logf("body: %s", string(body))
}

func doPost(t *testing.T, c *http.Client, baseURL, path string, body any) (int, []byte) {
	t.Helper()

	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, b
}

// TestAPI_CreateCamera tests POST /api/camera
// Request body based on test.yaml ffmpeg.cameras[0] config:
//   - id: camera1
//   - rtsp_url: rtsp://210.99.70.120:1935/live/cctv002.stream
//   - input_args: rtsp_transport=tcp, rtsp_flags=prefer_tcp, buffer_size=10485760, etc.
func TestAPI_CreateCamera(t *testing.T) {
	server, client := setupTestServer(t)

	name := "camera3"

	// Request body from test.yaml ffmpeg.cameras[0]
	reqBody := CameraCreateRequest{
		Table:   name,
		Name:    name,
		Desc:    "Test camera from ffmpeg config",
		RtspURL: "rtsp://210.99.70.120:1935/live/cctv002.stream",
		FFmpegOptions: []ReqKV{
			{K: "rtsp_transport", V: ptr("tcp")},
			{K: "rtsp_flags", V: ptr("prefer_tcp")},
			{K: "buffer_size", V: ptr("10485760")},
			{K: "max_delay", V: ptr("5000000")},
			{K: "probesize", V: ptr("10000000")},
			{K: "analyzeduration", V: ptr("10000000")},
			{K: "use_wallclock_as_timestamps", V: ptr("1")},
			{K: "c:v", V: ptr("copy")},
			{K: "f", V: ptr("dash")},
			{K: "seg_duration", V: ptr("5")},
			{K: "use_template", V: ptr("0")},
			{K: "use_timeline", V: ptr("0")},
		},
	}

	code, body := doPost(t, client, server.URL, "/api/camera", reqBody)
	t.Logf("CreateCamera response: %s", string(body))

	require.Equal(t, http.StatusOK, code, string(body))

	// Parse response
	var resp Response
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.True(t, resp.Success, resp.Reason)

	// Verify GetCameras returns the created camera
	code, body = doGet(t, client, server.URL, "/api/cameras", nil)
	require.Equal(t, http.StatusOK, code, string(body))

	var camerasResp Response
	err = json.Unmarshal(body, &camerasResp)
	require.NoError(t, err)
	require.True(t, camerasResp.Success)

	t.Logf("GetCameras response: %s", string(body))
}

// TestAPI_CreateCamera_WithDetectObjects tests POST /api/camera with detect_objects
func TestAPI_CreateCamera_WithDetectObjects(t *testing.T) {
	server, client := setupTestServer(t)

	reqBody := CameraCreateRequest{
		Table:         "test_camera_detect",
		Name:          "test_camera_detect",
		Desc:          "Camera with detection objects",
		RtspURL:       "rtsp://example.com/stream",
		DetectObjects: []string{"person", "car", "truck"},
	}

	code, body := doPost(t, client, server.URL, "/api/camera", reqBody)
	t.Logf("CreateCamera response: %s", string(body))

	require.Equal(t, http.StatusOK, code, string(body))

	// Parse response
	var resp Response
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.True(t, resp.Success, resp.Reason)

	t.Logf("CreateCamera success: %v", resp.Data)
}

// TestAPI_EnableDisableCamera tests POST /api/camera/:id/enable and /disable.
// 가짜 ffmpeg(sleep)를 사용하여 프로세스 시작/중지를 테스트.
// DB 불필요 - 카메라 설정 JSON 파일만 직접 생성.
func TestAPI_EnableDisableCamera(t *testing.T) {
	// 1. 가짜 ffmpeg 바이너리 생성 (sleep으로 대체)
	tmpBinDir := t.TempDir()
	fakeFFmpeg := filepath.Join(tmpBinDir, "fake_ffmpeg")
	err := os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexec sleep 300\n"), 0755)
	require.NoError(t, err)

	// 2. 테스트 서버 설정 (temp 디렉토리 사용)
	cfg, err := config.Load("../config/test.yaml")
	require.NoError(t, err)

	tmpDir := t.TempDir()
	cfg.Server.CameraDir = filepath.Join(tmpDir, "cameras")
	cfg.Server.MvsDir = filepath.Join(tmpDir, "mvs")
	cfg.Server.DataDir = filepath.Join(tmpDir, "data")

	machbase, err := db.NewMachbase(cfg.Machbase)
	require.NoError(t, err)

	svr, err := New(cfg.Server, machbase, fakeFFmpeg)
	require.NoError(t, err)

	server := httptest.NewServer(svr)
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = 30 * time.Second

	// 3. 카메라 설정 JSON 파일 직접 생성 (DB 테이블 생성 건너뜀)
	err = os.MkdirAll(cfg.Server.CameraDir, 0755)
	require.NoError(t, err)

	name := "test_enable_cam"
	camConfig := CameraCreateRequest{
		Table:   name,
		Name:    name,
		RtspURL: "rtsp://example.com/stream",
		FFmpegOptions: []ReqKV{
			{K: "rtsp_transport", V: ptr("tcp")},
			{K: "c:v", V: ptr("copy")},
			{K: "f", V: ptr("dash")},
		},
		OutputName: "manifest.mpd",
	}

	camJSON, err := json.MarshalIndent(camConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(cfg.Server.CameraDir, name+".json"), camJSON, 0644)
	require.NoError(t, err)

	// 4. Enable → 200 OK
	code, body := doPost(t, client, server.URL, "/api/camera/"+name+"/enable", nil)
	t.Logf("Enable response: %s", string(body))
	require.Equal(t, http.StatusOK, code, string(body))

	var resp Response
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.True(t, resp.Success, resp.Reason)

	// 5. Enable 중복 → 409 Conflict
	code, body = doPost(t, client, server.URL, "/api/camera/"+name+"/enable", nil)
	t.Logf("Enable again response: %s", string(body))
	require.Equal(t, http.StatusConflict, code, string(body))

	// 6. Disable → 200 OK
	code, body = doPost(t, client, server.URL, "/api/camera/"+name+"/disable", nil)
	t.Logf("Disable response: %s", string(body))
	require.Equal(t, http.StatusOK, code, string(body))

	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.True(t, resp.Success, resp.Reason)

	// 7. Disable 중복 → 404 Not Found
	code, body = doPost(t, client, server.URL, "/api/camera/"+name+"/disable", nil)
	t.Logf("Disable again response: %s", string(body))
	require.Equal(t, http.StatusNotFound, code, string(body))

	// 8. 존재하지 않는 카메라 Enable → 404
	code, body = doPost(t, client, server.URL, "/api/camera/nonexistent/enable", nil)
	t.Logf("Enable nonexistent response: %s", string(body))
	require.Equal(t, http.StatusNotFound, code, string(body))
}

// ptr is a helper function to create a pointer to a string
func ptr(s string) *string {
	return &s
}

// TestAPI_UploadAIResult tests POST /api/ai/result
func TestAPI_UploadAIResult(t *testing.T) {
	// Setup test server with temp directories
	cfg, err := config.Load("../config/test.yaml")
	require.NoError(t, err)

	tmpDir := t.TempDir()
	cfg.Server.CameraDir = filepath.Join(tmpDir, "cameras")
	cfg.Server.MvsDir = filepath.Join(tmpDir, "mvs")
	cfg.Server.DataDir = filepath.Join(tmpDir, "data")

	machbase, err := db.NewMachbase(cfg.Machbase)
	require.NoError(t, err)

	svr, err := New(cfg.Server, machbase)
	require.NoError(t, err)

	server := httptest.NewServer(svr)
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = 30 * time.Second

	// Create camera config directory
	err = os.MkdirAll(cfg.Server.CameraDir, 0755)
	require.NoError(t, err)

	cameraID := "test_camera_ai"
	tableName := cameraID + "_log"

	// Create camera config with SaveObjects=true and EventRule
	camConfig := CameraCreateRequest{
		Table:         cameraID,
		Name:          cameraID,
		SaveObjects:   true,
		DetectObjects: []string{"person", "car", "truck"},
		EventRule: []EventRule{
			{
				ID:         "rule1",
				Name:       "Test Rule 1",
				Expression: "person > 5",
				RecordMode: "ALL_MATCHES",
				Enabled:    true,
			},
			{
				ID:         "rule2",
				Name:       "Test Rule 2 (Disabled)",
				Expression: "car > 10",
				RecordMode: "EDGE_ONLY",
				Enabled:    false,
			},
		},
	}

	camJSON, err := json.MarshalIndent(camConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(cfg.Server.CameraDir, cameraID+".json"), camJSON, 0644)
	require.NoError(t, err)

	// Create camera tables in DB (main, _log, _event)
	ctx := t.Context()
	err = machbase.CreateCameraTables(ctx, cameraID)
	require.NoError(t, err)

	t.Run("Success - SaveObjects=true with enabled rules", func(t *testing.T) {
		reqBody := AIResultRequest{
			Table:     tableName,
			CameraID:  cameraID,
			ModelID:   "model_v1",
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
			Detections: map[string]float64{
				"person": 7.0,
				"car":    3.0,
				"truck":  2.0,
			},
			TotalObjects: 12,
		}

		code, body := doPost(t, client, server.URL, "/api/ai/result", reqBody)
		t.Logf("UploadAIResult response: %s", string(body))

		require.Equal(t, http.StatusOK, code, string(body))

		var resp Response
		err := json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.True(t, resp.Success, resp.Reason)
	})

	t.Run("BadRequest - Invalid timestamp format", func(t *testing.T) {
		reqBody := AIResultRequest{
			Table:     tableName,
			CameraID:  cameraID,
			ModelID:   "model_v1",
			Timestamp: "invalid-timestamp",
			Detections: map[string]float64{
				"person": 5.0,
			},
		}

		code, body := doPost(t, client, server.URL, "/api/ai/result", reqBody)
		t.Logf("Invalid timestamp response: %s", string(body))

		require.Equal(t, http.StatusBadRequest, code, string(body))

		var resp Response
		err := json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.False(t, resp.Success)
		require.Contains(t, resp.Reason, "timestamp")
	})

	t.Run("NotFound - Camera config not found", func(t *testing.T) {
		reqBody := AIResultRequest{
			Table:     "nonexistent_log",
			CameraID:  "nonexistent_camera",
			ModelID:   "model_v1",
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
			Detections: map[string]float64{
				"person": 5.0,
			},
		}

		code, body := doPost(t, client, server.URL, "/api/ai/result", reqBody)
		t.Logf("Nonexistent camera response: %s", string(body))

		require.Equal(t, http.StatusNotFound, code, string(body))

		var resp Response
		err := json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.False(t, resp.Success)
		require.Contains(t, resp.Reason, "not found")
	})

	t.Run("Success - SaveObjects=false (no OR_LOG insert)", func(t *testing.T) {
		// Create another camera with SaveObjects=false
		cameraID2 := "test_camera_no_save"
		tableName2 := cameraID2 + "_log"

		camConfig2 := CameraCreateRequest{
			Table:         cameraID2,
			Name:          cameraID2,
			SaveObjects:   false, // OR_LOG에 저장 안 함
			DetectObjects: []string{"person"},
			EventRule: []EventRule{
				{
					ID:         "rule1",
					Name:       "Test Rule",
					Expression: "person > 3",
					RecordMode: "ALL_MATCHES",
					Enabled:    true,
				},
			},
		}

		camJSON2, err := json.MarshalIndent(camConfig2, "", "  ")
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(cfg.Server.CameraDir, cameraID2+".json"), camJSON2, 0644)
		require.NoError(t, err)

		// Create tables
		err = machbase.CreateCameraTables(ctx, cameraID2)
		require.NoError(t, err)

		reqBody := AIResultRequest{
			Table:     tableName2,
			CameraID:  cameraID2,
			ModelID:   "model_v1",
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
			Detections: map[string]float64{
				"person": 5.0,
			},
		}

		code, body := doPost(t, client, server.URL, "/api/ai/result", reqBody)
		t.Logf("SaveObjects=false response: %s", string(body))

		require.Equal(t, http.StatusOK, code, string(body))

		var resp Response
		err = json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.True(t, resp.Success, resp.Reason)
	})

	t.Run("Success - No enabled rules (no EventLog insert)", func(t *testing.T) {
		// Create camera with all rules disabled
		cameraID3 := "test_camera_no_rules"
		tableName3 := cameraID3 + "_log"

		camConfig3 := CameraCreateRequest{
			Table:         cameraID3,
			Name:          cameraID3,
			SaveObjects:   true,
			DetectObjects: []string{"person"},
			EventRule: []EventRule{
				{
					ID:         "rule1",
					Name:       "Disabled Rule",
					Expression: "person > 3",
					RecordMode: "ALL_MATCHES",
					Enabled:    false, // disabled
				},
			},
		}

		camJSON3, err := json.MarshalIndent(camConfig3, "", "  ")
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(cfg.Server.CameraDir, cameraID3+".json"), camJSON3, 0644)
		require.NoError(t, err)

		// Create tables
		err = machbase.CreateCameraTables(ctx, cameraID3)
		require.NoError(t, err)

		reqBody := AIResultRequest{
			Table:     tableName3,
			CameraID:  cameraID3,
			ModelID:   "model_v1",
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
			Detections: map[string]float64{
				"person": 5.0,
			},
		}

		code, body := doPost(t, client, server.URL, "/api/ai/result", reqBody)
		t.Logf("No enabled rules response: %s", string(body))

		require.Equal(t, http.StatusOK, code, string(body))

		var resp Response
		err = json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.True(t, resp.Success, resp.Reason)
	})
}
