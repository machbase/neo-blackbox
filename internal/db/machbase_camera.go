package db

import (
	"context"
	"fmt"
)

// CreateCameraTables creates 3 tables for a camera: {name}, {name}_event, {name}_log
func (m *Machbase) CreateCameraTables(ctx context.Context, name string) error {
	// 1. Main camera table (video chunks)
	sqlMain := fmt.Sprintf(`CREATE TAG TABLE %s (
    name VARCHAR(128) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED,
    chunk_path VARCHAR(128)
) WITH ROLLUP`, name)

	if _, err := m.Query(ctx, sqlMain); err != nil {
		return fmt.Errorf("create table %s: %w", name, err)
	}

	// 2. Event table (DSL evaluation results)
	sqlEvent := fmt.Sprintf(`CREATE TAG TABLE %s_event (
    name VARCHAR(128) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE,
    expression_text VARCHAR(1024),
    used_counts_snapshot JSON
) METADATA (
    camera_id VARCHAR(64),
    rule_id VARCHAR(64)
)`, name)

	if _, err := m.Query(ctx, sqlEvent); err != nil {
		return fmt.Errorf("create table %s_event: %w", name, err)
	}

	// 3. Log table (detection counts per ident)
	sqlLog := fmt.Sprintf(`CREATE TAG TABLE %s_log (
    name VARCHAR(128) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE,
    model_id VARCHAR(64)
) METADATA (
    camera_id VARCHAR(64),
    ident VARCHAR(64)
)`, name)

	if _, err := m.Query(ctx, sqlLog); err != nil {
		return fmt.Errorf("create table %s_log: %w", name, err)
	}

	return nil
}

// CameraLogRow represents a detection log entry for {camera}_log table.
type CameraLogRow struct {
	Name     string  // camera_id.ident (ex: camera1.person)
	Time     int64   // nanoseconds
	Value    float64 // detection count
	ModelID  string
	CameraID string // metadata
	Ident    string // metadata
}

func (m *Machbase) InsertCameraLogs(ctx context.Context, table string, logs []CameraLogRow) error {
	columns := []string{"name", "time", "value", "model_id", "camera_id", "ident"}
	rows := make([][]any, len(logs))
	for i, l := range logs {
		rows[i] = []any{l.Name, l.Time, l.Value, l.ModelID, l.CameraID, l.Ident}
	}
	return m.WriteRows(ctx, table, columns, rows)
}

// CameraEventRow represents a DSL evaluation result for {camera}_event table.
type CameraEventRow struct {
	Name               string  // camera_id.rule_id
	Time               int64   // nanoseconds
	Value              float64 // 2=MATCH, 1=TRIGGER, 0=RESOLVE, -1=ERROR
	ExpressionText     string
	UsedCountsSnapshot string // JSON string
	CameraID           string // metadata
	RuleID             string // metadata
}

func (m *Machbase) InsertCameraEvents(ctx context.Context, table string, events []CameraEventRow) error {
	columns := []string{"name", "time", "value", "expression_text", "used_counts_snapshot", "camera_id", "rule_id"}
	rows := make([][]any, len(events))
	for i, e := range events {
		rows[i] = []any{e.Name, e.Time, e.Value, e.ExpressionText, e.UsedCountsSnapshot, e.CameraID, e.RuleID}
	}
	return m.WriteRows(ctx, table, columns, rows)
}

