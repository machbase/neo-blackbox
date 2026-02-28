package mediamtx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient는 httptest.Server를 가리키는 Client를 생성합니다.
// 테스트 종료 시 서버가 자동으로 닫힙니다.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &Client{baseURL: u, http: srv.Client()}
}

// --- AddPath ---

// TestAddPath_Success: AddPath가 올바른 HTTP 메서드, URL, JSON 바디로 요청하는지 검증합니다.
func TestAddPath_Success(t *testing.T) {
	var (
		capturedMethod string
		capturedPath   string
		capturedBody   PathConfig
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.WriteHeader(http.StatusOK)
	})

	client := newTestClient(t, handler)
	cfg := PathConfig{Source: "rtsp://cam1/live", SourceProtocol: PathSourceTCP}
	err := client.AddPath(context.Background(), "cam1", cfg)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, capturedMethod)
	assert.Equal(t, "/v3/config/paths/add/cam1", capturedPath)
	assert.Equal(t, "rtsp://cam1/live", capturedBody.Source)
	assert.Equal(t, PathSourceTCP, capturedBody.SourceProtocol)
}

// TestAddPath_ErrorResponse: 서버가 2xx가 아닌 응답을 반환하면 에러를 반환하는지 검증합니다.
func TestAddPath_ErrorResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "path already exists", http.StatusBadRequest)
	})

	client := newTestClient(t, handler)
	err := client.AddPath(context.Background(), "cam1", PathConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// --- UpdatePath ---

// TestUpdatePath_Success: UpdatePath가 PATCH 메서드와 올바른 URL 경로를 사용하는지 검증합니다.
func TestUpdatePath_Success(t *testing.T) {
	var capturedMethod, capturedPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	client := newTestClient(t, handler)
	err := client.UpdatePath(context.Background(), "cam2", PathConfig{Record: true})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, capturedMethod)
	assert.Equal(t, "/v3/config/paths/patch/cam2", capturedPath)
}

// TestUpdatePath_ErrorResponse: 서버 오류 시 에러를 반환하는지 검증합니다.
func TestUpdatePath_ErrorResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	client := newTestClient(t, handler)
	err := client.UpdatePath(context.Background(), "cam2", PathConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// --- RemovePath ---

// TestRemovePath_Success: RemovePath가 DELETE 메서드와 올바른 URL 경로를 사용하는지 검증합니다.
func TestRemovePath_Success(t *testing.T) {
	var capturedMethod, capturedPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	client := newTestClient(t, handler)
	err := client.RemovePath(context.Background(), "cam3")

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, capturedMethod)
	assert.Equal(t, "/v3/config/paths/delete/cam3", capturedPath)
}

// TestRemovePath_ErrorResponse: 서버가 에러 응답을 반환하면 에러를 반환하는지 검증합니다.
func TestRemovePath_ErrorResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client := newTestClient(t, handler)
	err := client.RemovePath(context.Background(), "cam3")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// --- GetPath ---

// TestGetPath_Found: path가 존재할 때 PathConfig를 올바르게 파싱하여 반환하는지 검증합니다.
func TestGetPath_Found(t *testing.T) {
	expected := PathConfig{
		Source:         "rtsp://cam1/live",
		SourceProtocol: PathSourceUDP,
		Record:         true,
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v3/config/paths/get/cam1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	})

	client := newTestClient(t, handler)
	got, err := client.GetPath(context.Background(), "cam1")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, expected.Source, got.Source)
	assert.Equal(t, expected.SourceProtocol, got.SourceProtocol)
	assert.Equal(t, expected.Record, got.Record)
}

// TestGetPath_NotFound: 서버가 404를 반환하면 nil, nil을 반환하는지 검증합니다.
func TestGetPath_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := newTestClient(t, handler)
	got, err := client.GetPath(context.Background(), "missing")

	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestGetPath_ErrorResponse: 서버가 5xx를 반환하면 에러를 반환하는지 검증합니다.
func TestGetPath_ErrorResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})

	client := newTestClient(t, handler)
	_, err := client.GetPath(context.Background(), "cam1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// --- GetPathStatus ---

// TestGetPathStatus_Ready: path가 Ready 상태일 때 올바른 PathStatus를 반환하는지 검증합니다.
func TestGetPathStatus_Ready(t *testing.T) {
	expected := PathStatus{Name: "cam1", Ready: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/paths/get/cam1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	})

	client := newTestClient(t, handler)
	got, err := client.GetPathStatus(context.Background(), "cam1")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "cam1", got.Name)
	assert.True(t, got.Ready)
}

// TestGetPathStatus_NotReady: path가 Ready가 아닐 때 Ready=false로 반환하는지 검증합니다.
func TestGetPathStatus_NotReady(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PathStatus{Name: "cam1", Ready: false})
	})

	client := newTestClient(t, handler)
	got, err := client.GetPathStatus(context.Background(), "cam1")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.Ready)
}

// TestGetPathStatus_NotFound: 서버가 404를 반환하면 nil, nil을 반환하는지 검증합니다.
func TestGetPathStatus_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := newTestClient(t, handler)
	got, err := client.GetPathStatus(context.Background(), "missing")

	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- AddOrUpdatePath ---

// TestAddOrUpdatePath_AddsWhenNotExist: path가 없으면 AddPath가 호출되는지 검증합니다.
func TestAddOrUpdatePath_AddsWhenNotExist(t *testing.T) {
	var addCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/get/cam1"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/add/cam1"):
			addCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	client := newTestClient(t, handler)
	err := client.AddOrUpdatePath(context.Background(), "cam1", PathConfig{})

	require.NoError(t, err)
	assert.True(t, addCalled, "path가 없으면 AddPath가 호출되어야 합니다")
}

// TestAddOrUpdatePath_UpdatesWhenExist: path가 이미 존재하면 UpdatePath가 호출되는지 검증합니다.
func TestAddOrUpdatePath_UpdatesWhenExist(t *testing.T) {
	var patchCalled bool
	existing := PathConfig{Source: "rtsp://old/stream"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/get/cam1"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(existing)
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/patch/cam1"):
			patchCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	client := newTestClient(t, handler)
	err := client.AddOrUpdatePath(context.Background(), "cam1", PathConfig{Source: "rtsp://new/stream"})

	require.NoError(t, err)
	assert.True(t, patchCalled, "path가 존재하면 UpdatePath가 호출되어야 합니다")
}

// --- WaitPathReady ---

// TestWaitPathReady_BecomesReady: 3번째 폴링에서 Ready가 되면 성공적으로 반환하는지 검증합니다.
func TestWaitPathReady_BecomesReady(t *testing.T) {
	var callCount int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		status := PathStatus{Name: "cam1", Ready: callCount >= 3}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	client := newTestClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := client.WaitPathReady(ctx, "cam1", 10*time.Millisecond)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Ready)
	assert.GreaterOrEqual(t, callCount, 3, "Ready가 되기까지 최소 3번 폴링해야 합니다")
}

// TestWaitPathReady_ContextTimeout: 컨텍스트 타임아웃 전에 Ready가 되지 않으면 에러를 반환하는지 검증합니다.
func TestWaitPathReady_ContextTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PathStatus{Name: "cam1", Ready: false})
	})

	client := newTestClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.WaitPathReady(ctx, "cam1", 10*time.Millisecond)

	require.Error(t, err)
	// ctx 만료 시 두 가지 에러 경로가 존재합니다:
	// 1. select에서 ctx.Done() 선택 → "timed out waiting for path..."
	// 2. ticker 선택 후 HTTP 요청 도중 ctx 만료 → "context deadline exceeded"
	isTimeoutErr := strings.Contains(err.Error(), "timed out") ||
		strings.Contains(err.Error(), "context deadline exceeded")
	assert.True(t, isTimeoutErr, "타임아웃 관련 에러여야 합니다: %v", err)
}

// --- 네트워크 에러 ---

// TestAddPath_NetworkError: 서버에 연결할 수 없으면 에러를 반환하는지 검증합니다.
func TestAddPath_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{baseURL: u, http: srv.Client()}
	srv.Close() // 즉시 닫아서 연결 거부 유발

	err = client.AddPath(context.Background(), "cam1", PathConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "do request")
}

// TestRemovePath_NetworkError: 서버에 연결할 수 없으면 에러를 반환하는지 검증합니다.
func TestRemovePath_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{baseURL: u, http: srv.Client()}
	srv.Close()

	err = client.RemovePath(context.Background(), "cam1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "do request")
}

// --- Malformed JSON ---

// TestGetPath_MalformedJSON: 서버가 유효하지 않은 JSON을 반환하면 decode 에러를 반환하는지 검증합니다.
func TestGetPath_MalformedJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	})

	client := newTestClient(t, handler)
	_, err := client.GetPath(context.Background(), "cam1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

// TestGetPathStatus_MalformedJSON: 서버가 유효하지 않은 JSON을 반환하면 decode 에러를 반환하는지 검증합니다.
func TestGetPathStatus_MalformedJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not-json`))
	})

	client := newTestClient(t, handler)
	_, err := client.GetPathStatus(context.Background(), "cam1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

// --- WaitPathReady 에러 전파 ---

// TestWaitPathReady_StatusError: GetPathStatus가 서버 에러를 반환하면 WaitPathReady도 즉시 에러를 반환하는지 검증합니다.
func TestWaitPathReady_StatusError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	client := newTestClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.WaitPathReady(ctx, "cam1", 10*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
