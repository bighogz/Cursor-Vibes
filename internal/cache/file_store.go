package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const defaultMaxAgeHours = 24

// FileStore implements DashboardStore using a single JSON file on disk.
// Good for: single-node deployments, local dev, make demo.
type FileStore struct {
	path   string
	maxAge time.Duration
}

func NewFileStore() *FileStore {
	p := filepath.Join("data", "dashboard_cache.json")
	if dir := os.Getenv("VIBES_DATA_DIR"); dir != "" {
		p = filepath.Join(dir, "dashboard_cache.json")
	}
	return &FileStore{path: p, maxAge: defaultMaxAgeHours * time.Hour}
}

type fileEnvelope struct {
	models.DashboardResult
	CachedAt string `json:"_cached_at"`
}

func (fs *FileStore) Load(allowStale bool) (*models.DashboardResult, bool) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, false
	}
	var env fileEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	if env.CachedAt == "" {
		return nil, false
	}
	cachedAt := parseTime(env.CachedAt)
	if !allowStale && time.Since(cachedAt) > fs.maxAge {
		return nil, false
	}
	r := env.DashboardResult
	return &r, true
}

func (fs *FileStore) Save(data *models.DashboardResult) error {
	dir := filepath.Dir(fs.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	env := fileEnvelope{
		DashboardResult: *data,
		CachedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fs.path, body, 0600)
}

func (fs *FileStore) LastUpdated() *time.Time {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil
	}
	var env struct {
		CachedAt string `json:"_cached_at"`
	}
	if err := json.Unmarshal(data, &env); err != nil || env.CachedAt == "" {
		return nil
	}
	t := parseTime(env.CachedAt)
	return &t
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05.999999", s)
	}
	return t
}
