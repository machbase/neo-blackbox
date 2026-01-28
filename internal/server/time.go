package server

import (
	"fmt"
	"strings"
	"time"
)

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// ParseTimeToken parses a time string to time.Time.
// Supports "now", ISO format, and date only format.
func ParseTimeToken(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}

	if strings.EqualFold(s, "now") {
		return time.Now(), nil
	}

	normalized := strings.ReplaceAll(s, "T", " ")
	normalized = strings.TrimSuffix(normalized, "Z")

	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, normalized, time.Local); err == nil {
			return t, nil
		}
	}

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Local(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local(), nil
	}

	return time.Time{}, fmt.Errorf("invalid time: %s", raw)
}

// FormatTime formats time.Time to ISO format string.
func FormatTime(t time.Time) string {
	s := t.Local().Format("2006-01-02T15:04:05.000000")
	if idx := strings.Index(s, "."); idx != -1 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
