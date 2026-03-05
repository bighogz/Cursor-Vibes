package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const maxAgeHours = 24

var cachePath = filepath.Join("data", "dashboard_cache.json")

func init() {
	if dir := os.Getenv("VIBES_DATA_DIR"); dir != "" {
		cachePath = filepath.Join(dir, "dashboard_cache.json")
	}
}

// cacheEnvelope wraps the dashboard result with a cache timestamp.
type cacheEnvelope struct {
	models.DashboardResult
	CachedAt string `json:"_cached_at"`
}

// ReadTyped returns a typed DashboardResult from the disk cache.
func ReadTyped(allowStale bool) (*models.DashboardResult, bool) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var env cacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	if env.CachedAt == "" {
		return nil, false
	}
	cachedAt, err := time.Parse(time.RFC3339, env.CachedAt)
	if err != nil {
		cachedAt, _ = time.Parse("2006-01-02T15:04:05.999999", env.CachedAt)
	}
	if !allowStale && time.Since(cachedAt) > maxAgeHours*time.Hour {
		return nil, false
	}
	r := env.DashboardResult
	return &r, true
}

// WriteTyped persists a typed DashboardResult to disk.
func WriteTyped(data *models.DashboardResult) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	env := cacheEnvelope{
		DashboardResult: *data,
		CachedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, body, 0600)
}

// Read returns raw map data for backward compatibility (e.g. CachedAt).
func Read(allowStale bool) (map[string]interface{}, bool) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

// Write persists any value as JSON (backward compat).
func Write(data interface{}) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, body, 0600)
}

func CachedAt() *time.Time {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	var env struct {
		CachedAt string `json:"_cached_at"`
	}
	if err := json.Unmarshal(data, &env); err != nil || env.CachedAt == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, env.CachedAt)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05.999999", env.CachedAt)
	}
	return &t
}
