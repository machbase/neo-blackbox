package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetSensors handles GET /api/sensors
func (h *Handler) GetSensors(c *gin.Context) {
	camera, err := h.requireTag(c, "tagname")
	if err != nil {
		return
	}

	sensors, err := h.db.GetSensors(c.Request.Context(), camera)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"camera": camera, "sensors": sensors})
}

// GetSensorData handles GET /api/sensor_data
func (h *Handler) GetSensorData(c *gin.Context) {
	param := strings.TrimSpace(c.Query("sensors"))
	if param == "" {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "missing required parameter: sensors"))
		return
	}

	var sensorIDs []string
	for _, p := range strings.Split(param, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		id, err := ValidateTag(p)
		if err != nil {
			h.writeError(c, NewAPIError(http.StatusBadRequest, err.Error()))
			return
		}
		sensorIDs = append(sensorIDs, id)
	}
	if len(sensorIDs) == 0 {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "sensors must include at least one sensor id"))
		return
	}

	startStr := strings.TrimSpace(c.Query("start"))
	endStr := strings.TrimSpace(c.Query("end"))
	if startStr == "" || endStr == "" {
		h.writeError(c, NewAPIError(http.StatusBadRequest, "missing required parameter: start or end"))
		return
	}

	start, err := ParseTimeToken(startStr)
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, err.Error()))
		return
	}
	end, err := ParseTimeToken(endStr)
	if err != nil {
		h.writeError(c, NewAPIError(http.StatusBadRequest, err.Error()))
		return
	}

	samples, err := h.db.GetSensorData(c.Request.Context(), sensorIDs, start, end)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sensors": sensorIDs, "samples": samples})
}
