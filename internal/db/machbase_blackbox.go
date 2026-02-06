package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blackbox-backend/internal/logger"
)

// CameraMetadataRow represents a camera metadata row.
type CameraMetadataRow struct {
	Name   string
	Prefix string
	FPS    *int
}

type metaRow struct {
	Name   string `json:"NAME"`
	Prefix string `json:"PREFIX"`
	FPS    *int   `json:"FPS"`
}

// Metadata fetches metadata for a given tag.
func (m *Machbase) Metadata(ctx context.Context, tag string) (*CameraMetadataRow, error) {
	safeTag := escapeSQLLiteral(tag)

	// Try first query
	sql := fmt.Sprintf("select name, prefix, fps from _blackbox3_meta where name = '%s'", safeTag)
	resp, err := m.Query(ctx, sql)
	if err == nil && resp.Data.Rows != nil {
		var rows []metaRow
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 {
			return &CameraMetadataRow{
				Name:   rows[0].Name,
				Prefix: rows[0].Prefix,
				FPS:    rows[0].FPS,
			}, nil
		}
	}

	// Try second query
	type metaRow2 struct {
		Prefix string `json:"PREFIX"`
		FPS    *int   `json:"FPS"`
	}
	sql = fmt.Sprintf("select prefix, fps from blackbox3 metadata where name = '%s'", safeTag)
	resp, err = m.Query(ctx, sql)
	if err == nil && resp.Data.Rows != nil {
		var rows []metaRow2
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 {
			return &CameraMetadataRow{
				Name:   tag,
				Prefix: rows[0].Prefix,
				FPS:    rows[0].FPS,
			}, nil
		}
	}

	return nil, nil
}

// CameraMetadata fetches all camera metadata.
func (m *Machbase) CameraMetadata(ctx context.Context) ([]CameraMetadataRow, error) {
	// Try first query
	sql := "select name, prefix, fps from _blackbox3_meta"
	resp, err := m.Query(ctx, sql)
	if err == nil && resp.Data.Rows != nil {
		var rows []metaRow
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 {
			results := make([]CameraMetadataRow, len(rows))
			for i, r := range rows {
				results[i] = CameraMetadataRow{
					Name:   r.Name,
					Prefix: r.Prefix,
					FPS:    r.FPS,
				}
			}
			return results, nil
		}
	}

	// Try second query
	sql = "select name, prefix, fps from blackbox3 metadata"
	resp, err = m.Query(ctx, sql)
	if err == nil && resp.Data.Rows != nil {
		var rows []metaRow
		if json.Unmarshal(resp.Data.Rows, &rows) == nil && len(rows) > 0 {
			results := make([]CameraMetadataRow, len(rows))
			for i, r := range rows {
				results[i] = CameraMetadataRow{
					Name:   r.Name,
					Prefix: r.Prefix,
					FPS:    r.FPS,
				}
			}
			return results, nil
		}
	}

	return nil, nil
}

// BlackboxStats represents blackbox statistics.
type BlackboxStats struct {
	Name    string
	MinTime time.Time
	MaxTime time.Time
}

// BlackboxStatsByTag fetches blackbox statistics for a camera.
// tableName: the table name (e.g., "camera2")
// cameraID: the camera ID stored in the 'name' column (e.g., "camera1")
func (m *Machbase) BlackboxStatsByTag(ctx context.Context, tableName string, cameraID string) (*BlackboxStats, error) {
	safeTable := escapeSQLLiteral(tableName)
	safeCameraID := escapeSQLLiteral(cameraID)
	// 카메라별 STAT 테이블: V${TABLE_NAME}_STAT
	// Note: Machbase system views use uppercase
	statTable := fmt.Sprintf("V$%s_STAT", strings.ToUpper(safeTable))
	sql := fmt.Sprintf(
		"select name, min_time, max_time from %s where name = '%s'",
		statTable, safeCameraID,
	)

	resp, err := m.Query(ctx, sql, WithTimeformat("ns"))
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Name    string `json:"NAME"`
		MinTime int64  `json:"MIN_TIME"`
		MaxTime int64  `json:"MAX_TIME"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	row := rows[0]
	return &BlackboxStats{
		Name:    row.Name,
		MinTime: time.Unix(0, row.MinTime),
		MaxTime: time.Unix(0, row.MaxTime),
	}, nil
}

// BlackboxTimeBounds fetches time bounds for a camera.
// tableName: the table name (e.g., "camera2")
// cameraID: the camera ID stored in the 'name' column (e.g., "camera1")
func (m *Machbase) BlackboxTimeBounds(ctx context.Context, tableName string, cameraID string) (*BlackboxStats, error) {
	safeTable := escapeSQLLiteral(tableName)
	safeCameraID := escapeSQLLiteral(cameraID)
	sql := fmt.Sprintf(
		"select min(time) as min_time, max(time) as max_time from %s where name = '%s'",
		safeTable, safeCameraID,
	)

	resp, err := m.Query(ctx, sql, WithTimeformat("ns"))
	if err != nil {
		return nil, err
	}

	var rows []struct {
		MinTime int64 `json:"MIN_TIME"`
		MaxTime int64 `json:"MAX_TIME"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	row := rows[0]
	if row.MinTime == 0 && row.MaxTime == 0 {
		return nil, nil
	}

	return &BlackboxStats{
		Name:    cameraID,
		MinTime: time.Unix(0, row.MinTime),
		MaxTime: time.Unix(0, row.MaxTime),
	}, nil
}

// BlackboxChunkInterval calculates the chunk interval for a camera.
// tableName: the table name (e.g., "camera2")
// cameraID: the camera ID stored in the 'name' column (e.g., "camera1")
func (m *Machbase) BlackboxChunkInterval(ctx context.Context, tableName string, cameraID string) (float64, error) {
	safeTable := escapeSQLLiteral(tableName)
	safeCameraID := escapeSQLLiteral(cameraID)
	sql := fmt.Sprintf(
		"select time from %s where name = '%s' order by time limit 2",
		safeTable, safeCameraID,
	)

	resp, err := m.Query(ctx, sql, WithTimeformat("ns"))
	if err != nil {
		return 0, err
	}

	var rows []struct {
		Time int64 `json:"TIME"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, nil
	}

	t1 := time.Unix(0, rows[0].Time)
	t2 := time.Unix(0, rows[1].Time)

	delta := t2.Sub(t1).Seconds()
	if delta <= 0 {
		return 0, nil
	}
	return delta, nil
}

// ListTags fetches all distinct tags.
// NOTE: Since each camera has its own table, this method needs to be implemented differently.
// Consider using GetCameras handler which reads from camera config files instead.
func (m *Machbase) ListTags(ctx context.Context) ([]string, error) {
	// TODO: Implement based on camera config files or system tables
	// For now, return empty list
	return []string{}, nil
}

// ChunkRecord represents a chunk record.
type ChunkRecord struct {
	ChunkPath string    // 파일 경로
	EntryTime time.Time
	Length    float64 // 길이 (초)
}

// ChunkRecordForTime fetches chunk record for a specific time.
// tableName: the table name (e.g., "camera2")
// cameraID: the camera ID stored in the 'name' column (e.g., "camera1")
func (m *Machbase) ChunkRecordForTime(ctx context.Context, tableName string, cameraID string, ts time.Time) (*ChunkRecord, error) {
	safeTable := escapeSQLLiteral(tableName)
	safeCameraID := escapeSQLLiteral(cameraID)
	upperNs := ts.UnixNano()
	endNs := upperNs + 6*1_000_000_000 // +6 seconds

	sql := fmt.Sprintf(
		"select /*+ SCAN_FORWARD(%s) */ time, value, chunk_path from %s "+
			"where name = '%s' and time >= %d and time <= %d order by time limit 1",
		safeTable, safeTable, safeCameraID, upperNs, endNs,
	)

	logger.GetLogger().Debugf("[CHUNK_QUERY] camera=%s, table=%s, start_ns=%d, end_ns=%d", cameraID, safeTable, upperNs, endNs)

	resp, err := m.Query(ctx, sql, WithTimeformat("ns"))
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Time      int64   `json:"TIME"`
		Value     float64 `json:"VALUE"`       // 길이 (초)
		ChunkPath string  `json:"CHUNK_PATH"` // 파일 경로
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	row := rows[0]
	return &ChunkRecord{
		ChunkPath: row.ChunkPath,
		EntryTime: time.Unix(0, row.Time),
		Length:    row.Value,
	}, nil
}

// RollupRow represents a rollup row.
type RollupRow struct {
	Time      time.Time
	SumLength float64
}

// CameraRollup fetches rollup data for a camera.
// tableName: the table name (e.g., "camera2")
// cameraID: the camera ID stored in the 'name' column (e.g., "camera1")
func (m *Machbase) CameraRollup(ctx context.Context, tableName string, cameraID string, minutes int, startNs, endNs int64) ([]RollupRow, error) {
	safeTable := escapeSQLLiteral(tableName)
	safeCameraID := escapeSQLLiteral(cameraID)
	sql := fmt.Sprintf(
		"select rollup('min', %d, time) as time, sum(value) as total_length "+
			"from %s where name = '%s' and time between %d and %d group by time order by time",
		minutes, safeTable, safeCameraID, startNs, endNs,
	)

	logger.GetLogger().Debugf("Machbase SQL (rollup): %s | minutes=%d | camera=%s | table=%s | start_ns=%d | end_ns=%d",
		sql, minutes, cameraID, safeTable, startNs, endNs)

	resp, err := m.Query(ctx, sql, WithTimeformat("ns"))
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Time        int64   `json:"TIME"`
		TotalLength float64 `json:"TOTAL_LENGTH"`
	}
	if err := json.Unmarshal(resp.Data.Rows, &rows); err != nil {
		return nil, err
	}

	result := make([]RollupRow, len(rows))
	for i, r := range rows {
		result[i] = RollupRow{
			Time:      time.Unix(0, r.Time),
			SumLength: r.TotalLength,
		}
	}
	return result, nil
}
