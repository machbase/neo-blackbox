package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"
)

// Blackbox types

type Camera struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type TimeRange struct {
	Camera        string  `json:"camera"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
	ChunkDuration float64 `json:"chunk_duration_seconds"`
	FPS           *int    `json:"fps,omitempty"`
}

type ChunkInfo struct {
	Camera string `json:"camera"`
	Time   string `json:"time"`
	Length int64  `json:"length"`
	Sign   int64  `json:"sign"`
}

type RollupRow struct {
	Time      string `json:"time"`
	SumLength int64  `json:"sum_length,omitempty"`
}

type RollupResult struct {
	Camera      string      `json:"camera"`
	Minutes     int         `json:"minutes"`
	StartTimeNS int64       `json:"start_time_ns"`
	EndTimeNS   int64       `json:"end_time_ns"`
	Start       string      `json:"start"`
	End         string      `json:"end"`
	Rows        []RollupRow `json:"rows"`
}

// GetCameras returns all available cameras.
func (m *Machbase) GetCameras(ctx context.Context) ([]Camera, error) {
	resp, err := m.Query(ctx, "SELECT name, prefix, fps FROM blackbox3 metadata")
	if err != nil {
		resp, err = m.Query(ctx, "SELECT name, prefix, fps FROM _blackbox3_meta")
		if err != nil {
			return nil, fmt.Errorf("get cameras: %w", err)
		}
	}

	var rows []struct {
		Name   string `json:"NAME"`
		Prefix string `json:"PREFIX"`
		FPS    int    `json:"FPS"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal rows: %w", err)
	}

	names := make(map[string]struct{})
	for _, r := range rows {
		if r.Name != "" {
			names[r.Name] = struct{}{}
		}
	}

	if len(names) == 0 {
		resp, err = m.Query(ctx, "SELECT DISTINCT name FROM blackbox3")
		if err != nil {
			return nil, fmt.Errorf("get cameras: %w", err)
		}

		var distinctRows []struct {
			Name string `json:"NAME"`
		}
		if err := json.Unmarshal(resp.Data.Rows, &distinctRows); err != nil {
			return nil, fmt.Errorf("unmarshal rows: %w", err)
		}
		for _, r := range distinctRows {
			if r.Name != "" {
				names[r.Name] = struct{}{}
			}
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("no cameras available")
	}

	cameras := make([]Camera, 0, len(names))
	for name := range names {
		cameras = append(cameras, Camera{ID: name, Label: name})
	}
	sort.Slice(cameras, func(i, j int) bool {
		return cameras[i].ID < cameras[j].ID
	})

	return cameras, nil
}

// GetTimeRange returns the time range for a camera.
func (m *Machbase) GetTimeRange(ctx context.Context, camera string) (*TimeRange, error) {
	tag := escapeSQLLiteral(camera)

	var start, end time.Time
	var found bool

	// Try stat table first
	query := fmt.Sprintf("SELECT name, min_time, max_time FROM v$blackbox3_stat WHERE name = '%s'", tag)
	if resp, err := m.Query(ctx, query, WithTimeformat("ns")); err == nil {
		var rows []struct {
			Name    string `json:"NAME"`
			MinTime int64  `json:"MIN_TIME"`
			MaxTime int64  `json:"MAX_TIME"`
		}
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 {
			start = time.Unix(0, rows[0].MinTime)
			end = time.Unix(0, rows[0].MaxTime)
			found = true
		}
	}

	// Fallback to direct query
	if !found || start.IsZero() || end.IsZero() {
		query = fmt.Sprintf("SELECT min(time) as min_time, max(time) as max_time FROM blackbox3 WHERE name = '%s'", tag)
		resp, err := m.Query(ctx, query, WithTimeformat("ns"))
		if err != nil {
			return nil, fmt.Errorf("get time range: %w", err)
		}

		var rows []struct {
			MinTime int64 `json:"MIN_TIME"`
			MaxTime int64 `json:"MAX_TIME"`
		}
		if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil || len(rows) == 0 {
			return nil, fmt.Errorf("no timeline entries for camera %s", camera)
		}
		start = time.Unix(0, rows[0].MinTime)
		end = time.Unix(0, rows[0].MaxTime)
	}

	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("no timeline entries for camera %s", camera)
	}

	// Get chunk duration
	chunkDuration := 5.0
	query = fmt.Sprintf("SELECT time FROM blackbox3 WHERE name = '%s' ORDER BY time LIMIT 2", tag)
	if resp, err := m.Query(ctx, query, WithTimeformat("ns")); err == nil {
		var rows []struct {
			Time int64 `json:"TIME"`
		}
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) >= 2 {
			t1 := time.Unix(0, rows[0].Time)
			t2 := time.Unix(0, rows[1].Time)
			if d := t2.Sub(t1).Seconds(); d > 0 {
				chunkDuration = d
			}
		}
	}

	// Get FPS
	var fps *int
	query = fmt.Sprintf("SELECT fps FROM blackbox3 metadata WHERE name = '%s'", tag)
	if resp, err := m.Query(ctx, query); err == nil {
		var rows []struct {
			FPS int `json:"FPS"`
		}
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 && rows[0].FPS > 0 {
			fps = &rows[0].FPS
		}
	}

	return &TimeRange{
		Camera:        camera,
		Start:         formatTime(start),
		End:           formatTime(end),
		ChunkDuration: chunkDuration,
		FPS:           fps,
	}, nil
}

// GetChunkInfo returns chunk info for a camera at a specific time.
func (m *Machbase) GetChunkInfo(ctx context.Context, camera string, t time.Time) (*ChunkInfo, error) {
	tag := escapeSQLLiteral(camera)
	startNS := t.UTC().UnixNano()
	endNS := startNS + 6*1e9 // 6 seconds forward

	query := fmt.Sprintf(`
		SELECT /*+ SCAN_FORWARD(blackbox3) */ time, length, value
		FROM blackbox3
		WHERE name = '%s' AND time >= %d AND time <= %d
		ORDER BY time LIMIT 1`,
		tag, startNS, endNS,
	)

	log.Printf("[CHUNK_QUERY] camera=%s, time=%s, start_ns=%d, end_ns=%d",
		camera, t.Format(time.RFC3339), startNS, endNS)

	resp, err := m.Query(ctx, query, WithTimeformat("ns"))
	if err != nil {
		return nil, fmt.Errorf("get chunk info: %w", err)
	}

	var rows []struct {
		Time   int64 `json:"TIME"`
		Length int64 `json:"LENGTH"`
		Value  int64 `json:"VALUE"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil || len(rows) == 0 {
		return nil, nil
	}

	return &ChunkInfo{
		Camera: camera,
		Time:   formatTime(time.Unix(0, rows[0].Time)),
		Length: rows[0].Length,
		Sign:   rows[0].Value,
	}, nil
}

// GetCameraRollup returns rollup data for a camera.
func (m *Machbase) GetCameraRollup(ctx context.Context, camera string, minutes int, startNS, endNS int64) (*RollupResult, error) {
	if minutes <= 0 {
		return nil, fmt.Errorf("minutes must be positive")
	}
	if startNS < 0 || endNS < 0 {
		return nil, fmt.Errorf("start and end time must be non-negative")
	}
	if startNS >= endNS {
		return nil, fmt.Errorf("start_time must be earlier than end_time")
	}

	tag := escapeSQLLiteral(camera)
	query := fmt.Sprintf(`
		SELECT rollup('min', %d, time) AS time, sum(length) AS total_length
		FROM blackbox3
		WHERE name = '%s' AND time BETWEEN %d AND %d
		GROUP BY time ORDER BY time`,
		minutes, tag, startNS, endNS,
	)

	resp, err := m.Query(ctx, query, WithTimeformat("ns"))
	if err != nil {
		return nil, fmt.Errorf("get camera rollup: %w", err)
	}

	var dbRows []struct {
		Time        int64 `json:"TIME"`
		TotalLength int64 `json:"TOTAL_LENGTH"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &dbRows); err != nil {
		return nil, fmt.Errorf("unmarshal rows: %w", err)
	}

	rows := make([]RollupRow, len(dbRows))
	for i, r := range dbRows {
		rows[i] = RollupRow{
			Time:      formatTime(time.Unix(0, r.Time)),
			SumLength: r.TotalLength,
		}
	}

	return &RollupResult{
		Camera:      camera,
		Minutes:     minutes,
		StartTimeNS: startNS,
		EndTimeNS:   endNS,
		Start:       formatTime(time.Unix(0, startNS)),
		End:         formatTime(time.Unix(0, endNS)),
		Rows:        rows,
	}, nil
}

// GetCameraPrefix returns the chunk file prefix for a camera.
func (m *Machbase) GetCameraPrefix(ctx context.Context, camera string) (string, error) {
	tag := escapeSQLLiteral(camera)

	query := fmt.Sprintf("SELECT prefix FROM blackbox3 metadata WHERE name = '%s'", tag)
	resp, err := m.Query(ctx, query)
	if err != nil {
		query = fmt.Sprintf("SELECT prefix FROM _blackbox3_meta WHERE name = '%s'", tag)
		resp, err = m.Query(ctx, query)
		if err != nil {
			return "chunk-stream", nil
		}
	}

	var rows []struct {
		Prefix string `json:"PREFIX"`
	}
	if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 && rows[0].Prefix != "" {
		return rows[0].Prefix, nil
	}

	return "chunk-stream", nil
}
