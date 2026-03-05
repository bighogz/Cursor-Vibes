package cache

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/models"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements DashboardStore using SQLite. Survives ephemeral
// filesystems (container restart) better than flat JSON because the DB can
// be mounted as a volume, and supports concurrent readers without file locking.
// Good for: multi-container, k8s PVC-backed, or any environment where a
// shared volume replaces local disk.
type SQLiteStore struct {
	db     *sql.DB
	maxAge time.Duration
}

func NewSQLiteStore() *SQLiteStore {
	dir := "data"
	if d := os.Getenv("VIBES_DATA_DIR"); d != "" {
		dir = d
	}
	os.MkdirAll(dir, 0755)
	dbPath := filepath.Join(dir, "cache.db")

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Fatalf("sqlite cache: open: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS dashboard_cache (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		data TEXT NOT NULL,
		cached_at TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatalf("sqlite cache: migrate: %v", err)
	}

	return &SQLiteStore{db: db, maxAge: defaultMaxAgeHours * time.Hour}
}

func (s *SQLiteStore) Load(allowStale bool) (*models.DashboardResult, bool) {
	var data, ts string
	err := s.db.QueryRow("SELECT data, cached_at FROM dashboard_cache WHERE id = 1").Scan(&data, &ts)
	if err != nil {
		return nil, false
	}
	cachedAt := parseTime(ts)
	if !allowStale && time.Since(cachedAt) > s.maxAge {
		return nil, false
	}
	var r models.DashboardResult
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, false
	}
	return &r, true
}

func (s *SQLiteStore) Save(d *models.DashboardResult) error {
	body, err := json.Marshal(d)
	if err != nil {
		return err
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO dashboard_cache (id, data, cached_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data, cached_at=excluded.cached_at`,
		string(body), ts,
	)
	return err
}

func (s *SQLiteStore) LastUpdated() *time.Time {
	var ts string
	if err := s.db.QueryRow("SELECT cached_at FROM dashboard_cache WHERE id = 1").Scan(&ts); err != nil {
		return nil
	}
	t := parseTime(ts)
	return &t
}
