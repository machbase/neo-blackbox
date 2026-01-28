package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"blackbox-backend/internal/config"
)

// Machbase is a client for Machbase HTTP API.
type Machbase struct {
	baseURL *url.URL
	client  *http.Client
}

// NewMachbase creates a new Machbase client.
func NewMachbase(cfg config.MachbaseConfig) (*Machbase, error) {
	cfg.ApplyDefaults()

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	u, err := url.Parse(fmt.Sprintf("%s://%s:%d", cfg.Scheme, cfg.Host, cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("invalid machbase config: %w", err)
	}

	return &Machbase{
		baseURL: u,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// Start initializes the Machbase client.
func (m *Machbase) Start() {}

// QueryResponse is the response from Machbase query API.
type QueryResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Data    struct {
		Columns []string        `json:"columns"`
		Types   []string        `json:"types"`
		Rows    json.RawMessage `json:"rows"`
	} `json:"data"`
}

func (m *Machbase) Query(ctx context.Context, sql string) (*QueryResponse, error) {
	u := m.baseURL.JoinPath("db", "query")

	q := u.Query()
	q.Set("q", sql)
	q.Set("rowsArray", "true")
	u.RawQuery = q.Encode()

	log.Printf("Machbase SQL: %s", sql)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	return m.do(req)
}

type writeRequest struct {
	Data struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	} `json:"data"`
}

// WriteOption configures write behavior.
type WriteOption func(*writeConfig)

type writeConfig struct {
	timeformat string
	tz         string
	method     string
}

// WriteRows writes rows to a table.
func (m *Machbase) WriteRows(ctx context.Context, table string, columns []string, rows [][]any, opts ...WriteOption) error {
	cfg := &writeConfig{
		timeformat: "ns",
		tz:         "UTC",
		method:     "insert",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if table == "" {
		return fmt.Errorf("table is empty")
	}
	if len(columns) == 0 {
		return fmt.Errorf("columns is empty")
	}
	if len(rows) == 0 {
		return fmt.Errorf("rows is empty")
	}

	u := m.baseURL.JoinPath("db", "write", table)

	q := u.Query()
	q.Set("timeformat", cfg.timeformat)
	q.Set("tz", cfg.tz)
	q.Set("method", cfg.method)
	u.RawQuery = q.Encode()

	var payload writeRequest
	payload.Data.Columns = columns
	payload.Data.Rows = rows

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	_, err = m.do(req)
	return err
}

const maxResponseBytes int64 = 8 << 20 // 8 MiB

func (m *Machbase) do(req *http.Request) (*QueryResponse, error) {
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response too large: limit %d bytes", maxResponseBytes)
	}

	var out QueryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("query failed: %s", out.Reason)
	}

	return &out, nil
}

// Helper functions

func escapeSQLLiteral(v string) string {
	return strings.ReplaceAll(v, "'", "''")
}

func formatTime(t time.Time) string {
	s := t.Local().Format("2006-01-02T15:04:05.000000")
	if idx := strings.Index(s, "."); idx != -1 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// parseDateTime parses a date time value from Machbase.
func parseDateTime(val any) (time.Time, bool) {
	if val == nil {
		return time.Time{}, false
	}
	switch v := val.(type) {
	case float64:
		return time.Unix(0, int64(v)*1000), true
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t, true
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(0, n), true
		}
	}
	return time.Time{}, false
}

// toFloat64 converts a value to float64.
func toFloat64(val any) (float64, bool) {
	if val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
