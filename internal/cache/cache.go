package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxAgeHours = 24

var cachePath = filepath.Join("data", "dashboard_cache.json")

func init() {
	if dir := os.Getenv("VIBES_DATA_DIR"); dir != "" {
		cachePath = filepath.Join(dir, "dashboard_cache.json")
	}
}

func Read(allowStale bool) (map[string]interface{}, bool) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	ts, ok := m["_cached_at"].(string)
	if !ok || ts == "" {
		return nil, false
	}
	cachedAt, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		cachedAt, _ = time.Parse("2006-01-02T15:04:05.999999", ts)
	}
	if !allowStale && time.Since(cachedAt) > maxAgeHours*time.Hour {
		return nil, false
	}
	return m, true
}

func Write(data map[string]interface{}) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["_cached_at"] = time.Now().UTC().Format(time.RFC3339)
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, body, 0600)
}

func CachedAt() *time.Time {
	m, ok := Read(true)
	if !ok {
		return nil
	}
	ts, ok := m["_cached_at"].(string)
	if !ok || ts == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05.999999", ts)
	}
	return &t
}
