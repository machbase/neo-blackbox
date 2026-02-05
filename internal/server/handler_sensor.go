package server

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetSensors handles GET /api/sensors.
func (h *Handler) GetSensors(c *gin.Context) {
	tick := time.Now()

	var req GetSensorsRequest
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
	var sensorIDs []string

	rawNames, err := h.machbase.SensorNames(ctx)
	if err == nil && len(rawNames) > 0 {
		for _, tag := range rawNames {
			sensorID := sensorKeyFromTag(camera, tag)
			if sensorID != "" {
				sensorIDs = append(sensorIDs, sensorID)
			}
		}
	}

	if len(sensorIDs) == 0 {
		sensorIDs = append(sensorIDs, defaultSensorNames...)
	}

	sensorIDs = uniqueStrings(sensorIDs)
	sortSensorIDs(sensorIDs)

	sensors := make([]Sensor, len(sensorIDs))
	for i, sensorID := range sensorIDs {
		label := defaultSensorLabels[sensorID]
		if label == "" {
			label = strings.ReplaceAll(sensorID, "_", " ")
			label = strings.Title(label)
		}
		sensors[i] = Sensor{ID: sensorID, Label: label}
	}

	successResponse(c, tick, GetSensorsResponse{
		Camera:  camera,
		Sensors: sensors,
	})
}

// GetSensorData handles GET /api/sensor_data.
func (h *Handler) GetSensorData(c *gin.Context) {
	tick := time.Now()

	var req GetSensorDataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameters")
		return
	}

	if req.Sensors == "" {
		errorResponse(c, tick, http.StatusBadRequest, "Missing required parameter 'sensors'")
		return
	}

	sensorTokens := strings.Split(req.Sensors, ",")
	var sensorIDs []string
	for _, token := range sensorTokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		sanitized, err := sanitizeTag(token)
		if err != nil {
			errorResponse(c, tick, http.StatusBadRequest, err.Error())
			return
		}
		sensorIDs = append(sensorIDs, sanitized)
	}

	if len(sensorIDs) == 0 {
		errorResponse(c, tick, http.StatusBadRequest, "Parameter 'sensors' must include at least one sensor id")
		return
	}

	startDt, err := parseTimeToken(req.Start)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Invalid start time format")
		return
	}

	endDt, err := parseTimeToken(req.End)
	if err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "Invalid end time format")
		return
	}

	if startDt.After(endDt) {
		errorResponse(c, tick, http.StatusBadRequest, "Start time must be earlier than end time")
		return
	}

	ctx := c.Request.Context()
	rows, err := h.machbase.SensorRows(ctx, sensorIDs, startDt, endDt)
	if err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "Failed to fetch sensor data")
		return
	}

	grouped := make(map[time.Time]map[string]float64)
	for _, row := range rows {
		matchedID := matchSensorID(row.Name, sensorIDs)
		if matchedID == "" {
			continue
		}
		if row.Time.Before(startDt) || row.Time.After(endDt) {
			continue
		}
		if grouped[row.Time] == nil {
			grouped[row.Time] = make(map[string]float64)
		}
		grouped[row.Time][matchedID] = row.Value
	}

	var times []time.Time
	for t := range grouped {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	samples := make([]SensorSample, len(times))
	for i, t := range times {
		samples[i] = SensorSample{
			Time:   formatTime(t),
			Values: grouped[t],
		}
	}

	successResponse(c, tick, GetSensorDataResponse{
		Sensors: sensorIDs,
		Samples: samples,
	})
}
