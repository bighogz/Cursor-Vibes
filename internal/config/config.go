package config

import (
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

var cfg struct {
	once   sync.Once
	loaded bool
}

// Load reads .env and populates exported vars. Safe to call multiple times;
// only the first call has effect. Called automatically by Cfg(), but callers
// may invoke it explicitly from main() for clarity.
func Load() {
	cfg.once.Do(func() {
		godotenv.Load(".env")

		FMPAPIKey = get("FMP_API_KEY")
		EODHDAPIKey = get("EODHD_API_KEY")
		SECAPIKey = get("SEC_API_KEY")
		FinancialDatasetsKey = get("FINANCIAL_DATASETS_API_KEY")
		FMPFreeTier = getBool("FMP_FREE_TIER", "false")
		AdminAPIKey = get("ADMIN_API_KEY")
		AIServiceURL = getDefault("AI_SERVICE_URL", "http://localhost:8001")

		cfg.loaded = true
	})
}

func get(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// Get reads an env var. Ensures .env is loaded first.
func Get(key string) string {
	Load()
	return get(key)
}

func getDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getBool(key, defaultVal string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		v = defaultVal
	}
	return v == "1" || v == "true" || v == "yes"
}

// GetBool reads a boolean env var. Ensures .env is loaded first.
func GetBool(key, defaultVal string) bool {
	Load()
	return getBool(key, defaultVal)
}

// Exported config values. Populated by Load().
var (
	FMPAPIKey            string
	EODHDAPIKey          string
	SECAPIKey            string
	FinancialDatasetsKey string

	AnomalyStdThreshold    = 2.0
	BaselineDays           = 730
	CurrentWindowDays      = 30
	MinBaselineWeeks       = 3
	VolumeWeight           = 0.4
	BreadthWeight          = 0.3
	AccelerationWeight     = 0.3

	FMPFreeTier  bool
	AdminAPIKey  string
	AIServiceURL string
)
