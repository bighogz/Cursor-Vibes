package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load(".env")
}

func Get(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func GetBool(key, defaultVal string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		v = defaultVal
	}
	return v == "1" || v == "true" || v == "yes"
}

var (
	FMPAPIKey           = Get("FMP_API_KEY")
	EODHDAPIKey         = Get("EODHD_API_KEY")
	SECAPIKey           = Get("SEC_API_KEY")
	FinancialDatasetsKey = Get("FINANCIAL_DATASETS_API_KEY")

	AnomalyStdThreshold = 2.0
	BaselineDays        = 365
	CurrentWindowDays   = 30
	MinBaselinePoints   = 5

	FMPFreeTier = GetBool("FMP_FREE_TIER", "false")

	AdminAPIKey = Get("ADMIN_API_KEY")
)
