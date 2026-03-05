package cache

import (
	"os"

	"github.com/bighogz/Cursor-Vibes/internal/models"
	"time"
)

// Global store used by the backward-compatible functions.
// Initialized lazily via getStore().
var globalStore DashboardStore

func getStore() DashboardStore {
	if globalStore != nil {
		return globalStore
	}
	backend := os.Getenv("VIBES_CACHE_BACKEND")
	switch backend {
	case "sqlite":
		globalStore = NewSQLiteStore()
	default:
		globalStore = NewFileStore()
	}
	return globalStore
}

// SetStore allows callers (e.g. main.go) to inject a specific store.
func SetStore(s DashboardStore) {
	globalStore = s
}

// --- Backward-compatible API used by existing callers ---

func ReadTyped(allowStale bool) (*models.DashboardResult, bool) {
	return getStore().Load(allowStale)
}

func WriteTyped(data *models.DashboardResult) error {
	return getStore().Save(data)
}

func CachedAt() *time.Time {
	return getStore().LastUpdated()
}
