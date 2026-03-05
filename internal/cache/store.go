// Package cache defines the DashboardStore interface and provides pluggable
// implementations. FileStore is the default (single-binary, zero deps).
// SQLiteStore demonstrates the pattern for clustered/ephemeral environments.
//
// The interface follows the Dependency Inversion principle: callers depend on
// the Store abstraction, not the storage mechanism. Swapping in Redis, S3,
// or any other backend requires only implementing three methods.
package cache

import (
	"github.com/bighogz/Cursor-Vibes/internal/models"
	"time"
)

// DashboardStore abstracts dashboard persistence. Implementations must be
// safe for concurrent use.
type DashboardStore interface {
	// Load returns the most recent dashboard. If allowStale is false,
	// returns nil when the cached data exceeds MaxAge.
	Load(allowStale bool) (*models.DashboardResult, bool)

	// Save persists a dashboard snapshot.
	Save(data *models.DashboardResult) error

	// LastUpdated returns when the cache was last written, or nil.
	LastUpdated() *time.Time
}

// DefaultStore returns a DashboardStore based on VIBES_CACHE_BACKEND.
// Supported values: "file" (default), "sqlite".
func DefaultStore() DashboardStore {
	return NewFileStore()
}
