package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"neo-blackbox/internal/logger"

	"github.com/gin-gonic/gin"
)

// GetEventRules handles GET /api/event_rule/:camera_id.
// 특정 카메라의 모든 EventRule 배열을 조회.
func (h *Handler) GetEventRules(c *gin.Context) {
	tick := time.Now()

	cameraID := c.Param("camera_id")
	if cameraID == "" {
		errorResponse(c, tick, http.StatusBadRequest, "camera_id is required")
		return
	}

	// 카메라 설정 파일 읽기
	cameraPath := filepath.Join(h.cameraDir, cameraID+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", cameraID))
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

	// nil slice를 빈 배열로 변환
	eventRules := camera.EventRule
	if eventRules == nil {
		eventRules = []EventRule{}
	}

	successResponse(c, tick, map[string]any{
		"camera_id":   cameraID,
		"event_rules": eventRules,
	})
}

// PostEventRulesRequest represents the request body for adding a new event rule.
type PostEventRulesRequest struct {
	CameraID string    `json:"camera_id" binding:"required"`
	Rule     EventRule `json:"rule" binding:"required"`
}

// PostEventRules handles POST /api/event_rule.
// 새로운 EventRule을 카메라 설정에 추가.
func (h *Handler) PostEventRules(c *gin.Context) {
	tick := time.Now()

	var req PostEventRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "bad request parameter")
		return
	}

	// 카메라 설정 파일 읽기
	cameraPath := filepath.Join(h.cameraDir, req.CameraID+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", req.CameraID))
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

	// rule_id 중복 체크
	for _, existing := range camera.EventRule {
		if existing.ID == req.Rule.ID {
			errorResponse(c, tick, http.StatusConflict, fmt.Sprintf("rule_id '%s' already exists", req.Rule.ID))
			return
		}
	}

	// expression_text를 소문자로 변환
	req.Rule.Expression = strings.ToLower(req.Rule.Expression)

	// record_mode 대문자 정규화 및 유효성 검사
	req.Rule.RecordMode = strings.ToUpper(req.Rule.RecordMode)
	if req.Rule.RecordMode != "ALL_MATCHES" && req.Rule.RecordMode != "EDGE_ONLY" {
		logger.GetLogger().Errorf("PostEventRules[%s]: invalid record_mode '%s'", req.CameraID, req.Rule.RecordMode)
		errorResponse(c, tick, http.StatusBadRequest, fmt.Sprintf("invalid record_mode '%s': must be 'ALL_MATCHES' or 'EDGE_ONLY'", req.Rule.RecordMode))
		return
	}

	// EventRule 추가
	camera.EventRule = append(camera.EventRule, req.Rule)

	// 이벤트룰 추가 시 event 테이블 생성
	if err := h.machbase.CreateCameraEventTable(c.Request.Context(), camera.Table); err != nil {
		logger.GetLogger().Warnf("PostEventRules[%s]: failed to create event table (may already exist): %v", req.CameraID, err)
		// 테이블 생성 실패해도 계속 진행 (이미 존재할 수 있음)
	}

	// 파일 저장
	cameraJSON, err := json.MarshalIndent(camera, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal camera config")
		return
	}

	if err := os.WriteFile(cameraPath, cameraJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write camera config file")
		return
	}

	// Event rules 캐시 갱신
	h.refreshCameraConfigCache(req.CameraID)

	successResponse(c, tick, map[string]any{
		"camera_id": req.CameraID,
		"rule":      req.Rule,
	})
}

// UpdateEventRules handles POST /api/event_rule/:camera_id/:rule_id.
// 기존 EventRule을 수정.
func (h *Handler) UpdateEventRules(c *gin.Context) {
	tick := time.Now()

	// URL 파라미터에서 camera_id, rule_id 가져오기
	cameraID := c.Param("camera_id")
	ruleID := c.Param("rule_id")

	if cameraID == "" {
		errorResponse(c, tick, http.StatusBadRequest, "camera_id is required")
		return
	}
	if ruleID == "" {
		errorResponse(c, tick, http.StatusBadRequest, "rule_id is required")
		return
	}

	var rule EventRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "bad request parameter")
		return
	}

	// 카메라 설정 파일 읽기
	cameraPath := filepath.Join(h.cameraDir, cameraID+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", cameraID))
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

	// expression_text를 소문자로 변환
	rule.Expression = strings.ToLower(rule.Expression)

	// record_mode 대문자 정규화 및 유효성 검사
	rule.RecordMode = strings.ToUpper(rule.RecordMode)
	if rule.RecordMode != "ALL_MATCHES" && rule.RecordMode != "EDGE_ONLY" {
		logger.GetLogger().Errorf("UpdateEventRules[%s/%s]: invalid record_mode '%s'", cameraID, ruleID, rule.RecordMode)
		errorResponse(c, tick, http.StatusBadRequest, fmt.Sprintf("invalid record_mode '%s': must be 'ALL_MATCHES' or 'EDGE_ONLY'", rule.RecordMode))
		return
	}

	// rule_id 찾아서 수정
	found := false
	for i, existing := range camera.EventRule {
		if existing.ID == ruleID {
			// rule_id는 변경 불가 (URL에서 지정된 것을 유지)
			rule.ID = ruleID
			camera.EventRule[i] = rule
			found = true
			break
		}
	}

	if !found {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("rule_id '%s' not found", ruleID))
		return
	}

	// 파일 저장
	cameraJSON, err := json.MarshalIndent(camera, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal camera config")
		return
	}

	if err := os.WriteFile(cameraPath, cameraJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write camera config file")
		return
	}

	// Event rules 캐시 갱신
	h.refreshCameraConfigCache(cameraID)

	successResponse(c, tick, map[string]any{
		"camera_id": cameraID,
		"rule":      rule,
	})
}

// DeleteEventRules handles DELETE /api/event_rule/:camera_id/:rule_id.
// 특정 EventRule을 삭제.
func (h *Handler) DeleteEventRules(c *gin.Context) {
	tick := time.Now()

	cameraID := c.Param("camera_id")
	ruleID := c.Param("rule_id")

	if cameraID == "" {
		errorResponse(c, tick, http.StatusBadRequest, "camera_id is required")
		return
	}
	if ruleID == "" {
		errorResponse(c, tick, http.StatusBadRequest, "rule_id is required")
		return
	}

	// 카메라 설정 파일 읽기
	cameraPath := filepath.Join(h.cameraDir, cameraID+".json")
	data, err := os.ReadFile(cameraPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("camera '%s' not found", cameraID))
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

	// rule_id 찾아서 삭제
	found := false
	newRules := make([]EventRule, 0, len(camera.EventRule))
	for _, rule := range camera.EventRule {
		if rule.ID == ruleID {
			found = true
			continue // 이 rule은 스킵 (삭제)
		}
		newRules = append(newRules, rule)
	}

	if !found {
		errorResponse(c, tick, http.StatusNotFound, fmt.Sprintf("rule_id '%s' not found", ruleID))
		return
	}

	camera.EventRule = newRules

	// 파일 저장
	cameraJSON, err := json.MarshalIndent(camera, "", "  ")
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to marshal camera config")
		return
	}

	if err := os.WriteFile(cameraPath, cameraJSON, 0644); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to write camera config file")
		return
	}

	// Event rules 캐시 갱신
	h.refreshCameraConfigCache(cameraID)

	successResponse(c, tick, map[string]string{
		"camera_id": cameraID,
		"rule_id":   ruleID,
	})
}
