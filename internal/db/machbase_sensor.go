package db

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Sensor types

type Sensor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type SensorSample struct {
	Time   string             `json:"time"`
	Values map[string]float64 `json:"values"`
}

// sensorNumPattern matches sensor-N pattern for sorting.
var sensorNumPattern = regexp.MustCompile(`^sensor-(\d+)$`)

// GetSensors returns available sensors for a camera.
func (m *Machbase) GetSensors(ctx context.Context, camera string) ([]Sensor, error) {
	rows, err := m.selectRows(ctx, "SELECT name FROM _sensor3_meta ORDER BY name")
	if err != nil {
		rows, err = m.selectRows(ctx, "SELECT DISTINCT name FROM sensor3 ORDER BY name")
		if err != nil {
			return defaultSensors(), nil
		}
	}

	ids := make(map[string]struct{})
	for _, row := range rows {
		if len(row) > 0 {
			if name, ok := row[0].(string); ok && name != "" {
				if id := extractSensorID(camera, name); id != "" {
					ids[id] = struct{}{}
				}
			}
		}
	}

	if len(ids) == 0 {
		return defaultSensors(), nil
	}

	sensors := make([]Sensor, 0, len(ids))
	for id := range ids {
		sensors = append(sensors, Sensor{
			ID:    id,
			Label: sensorLabel(id),
		})
	}
	sort.Slice(sensors, func(i, j int) bool {
		return sensorSortKey(sensors[i].ID) < sensorSortKey(sensors[j].ID)
	})

	return sensors, nil
}

// GetSensorData returns sensor data for the given sensors and time range.
func (m *Machbase) GetSensorData(ctx context.Context, sensorIDs []string, start, end time.Time) ([]SensorSample, error) {
	if len(sensorIDs) == 0 {
		return []SensorSample{}, nil
	}
	if start.After(end) {
		return nil, fmt.Errorf("start time must be earlier than end time")
	}

	quoted := make([]string, len(sensorIDs))
	for i, id := range sensorIDs {
		quoted[i] = fmt.Sprintf("'%s'", escapeSQLLiteral(id))
	}

	query := fmt.Sprintf(`
		SELECT name, time, value
		FROM sensor3
		WHERE name IN (%s) AND time BETWEEN %d AND %d
		ORDER BY time`,
		strings.Join(quoted, ", "),
		start.UTC().UnixNano(),
		end.UTC().UnixNano(),
	)

	rows, err := m.selectRows(ctx, query, QueryOptions{Timeformat: "us"})
	if err != nil {
		return nil, fmt.Errorf("get sensor data: %w", err)
	}

	grouped := make(map[int64]map[string]float64)
	timeMap := make(map[int64]time.Time)

	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		name, _ := row[0].(string)
		t, ok := parseDateTime(row[1])
		if !ok {
			continue
		}
		val, ok := toFloat64(row[2])
		if !ok {
			continue
		}

		matchedID := matchSensorID(name, sensorIDs)
		if matchedID == "" {
			continue
		}

		key := t.UnixNano()
		if grouped[key] == nil {
			grouped[key] = make(map[string]float64)
			timeMap[key] = t
		}
		grouped[key][matchedID] = val
	}

	keys := make([]int64, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	samples := make([]SensorSample, 0, len(keys))
	for _, k := range keys {
		samples = append(samples, SensorSample{
			Time:   formatTime(timeMap[k]),
			Values: grouped[k],
		})
	}

	return samples, nil
}

// Sensor helper functions

func defaultSensors() []Sensor {
	sensors := make([]Sensor, 10)
	for i := range sensors {
		id := fmt.Sprintf("sensor-%d", i+1)
		sensors[i] = Sensor{ID: id, Label: fmt.Sprintf("Sensor %d", i+1)}
	}
	return sensors
}

func extractSensorID(camera, tag string) string {
	if tag == "" {
		return ""
	}
	for _, sep := range []string{":", "."} {
		if prefix := camera + sep; strings.HasPrefix(tag, prefix) {
			return tag[len(prefix):]
		}
	}
	return tag
}

func sensorLabel(id string) string {
	if strings.HasPrefix(id, "sensor-") {
		return "Sensor " + strings.TrimPrefix(id, "sensor-")
	}
	s := strings.ReplaceAll(id, "_", " ")
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func sensorSortKey(id string) string {
	if m := sensorNumPattern.FindStringSubmatch(id); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return fmt.Sprintf("0-%08d", n)
	}
	return "1-" + id
}

func matchSensorID(tagName string, sensorIDs []string) string {
	for _, id := range sensorIDs {
		if tagName == id {
			return id
		}
		for _, sep := range []string{":", "."} {
			if strings.HasSuffix(tagName, sep+id) {
				return id
			}
		}
	}
	return ""
}
