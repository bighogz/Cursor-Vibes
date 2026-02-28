// Package rustclient invokes the Rust vibes-anomaly binary for anomaly detection.
package rustclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

// AnomalySignal matches Rust output.
type AnomalySignal struct {
	Ticker            string  `json:"ticker"`
	CurrentSharesSold float64 `json:"current_shares_sold"`
	BaselineMean      float64 `json:"baseline_mean"`
	BaselineStd       float64 `json:"baseline_std"`
	ZScore            float64 `json:"z_score"`
	IsAnomaly         bool    `json:"is_anomaly"`
}

// rustInput is sent to vibes-anomaly stdin.
type rustInput struct {
	Records []models.InsiderSellRecord `json:"records"`
	Params  struct {
		BaselineDays        int     `json:"baseline_days"`
		CurrentDays         int     `json:"current_days"`
		StdThreshold        float64 `json:"std_threshold"`
		MinBaselinePoints   int     `json:"min_baseline_points"`
		AsOf                string  `json:"as_of"`
	} `json:"params"`
}

// rustOutput is read from vibes-anomaly stdout.
type rustOutput struct {
	Signals []AnomalySignal `json:"signals"`
}

var binPath string

func init() {
	if p := os.Getenv("VIBES_ANOMALY_BIN"); p != "" {
		if validateBinPath(p) {
			binPath = p
		}
		return
	}
	cwd, _ := os.Getwd()
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	candidates := []string{
		filepath.Join(execDir, "vibes-anomaly"),                     // next to api binary (bin/)
		filepath.Join(execDir, "..", "rust-core", "target", "release", "vibes-anomaly"),
		filepath.Join(cwd, "bin", "vibes-anomaly"),
		filepath.Join(cwd, "rust-core", "target", "release", "vibes-anomaly"),
		filepath.Join(cwd, "target", "release", "vibes-anomaly"),
		"vibes-anomaly",
	}
	for _, p := range candidates {
		if p == "vibes-anomaly" {
			if p2, err := exec.LookPath(p); err == nil {
				binPath = p2
				return
			}
			continue
		}
		if _, err := os.Stat(p); err == nil {
			binPath = p
			return
		}
	}
	binPath = ""
}

// validateBinPath ensures VIBES_ANOMALY_BIN is an absolute path under the project root.
func validateBinPath(p string) bool {
	if p == "" {
		return false
	}
	if !filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	projectRoot, err := os.Getwd()
	if err != nil {
		projectRoot = filepath.Dir(os.Args[0])
	}
	projectRoot = filepath.Clean(projectRoot)
	prefix := projectRoot + string(filepath.Separator)
	return strings.HasPrefix(clean, prefix) || clean == projectRoot
}

// Available returns true if the Rust binary is found.
func Available() bool {
	return binPath != ""
}

// ComputeAnomalySignals runs vibes-anomaly with the given records and params.
// Returns (signals, nil) on success, or (nil, error) on failure.
func ComputeAnomalySignals(
	records []models.InsiderSellRecord,
	baselineDays, currentDays int,
	stdThreshold float64,
	asOf string,
) ([]AnomalySignal, error) {
	if !Available() {
		return nil, fmt.Errorf("vibes-anomaly binary not found")
	}
	input := rustInput{
		Records: records,
	}
	input.Params.BaselineDays = baselineDays
	input.Params.CurrentDays = currentDays
	input.Params.StdThreshold = stdThreshold
	input.Params.MinBaselinePoints = config.MinBaselinePoints
	input.Params.AsOf = asOf

	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(body)
	var out bytes.Buffer
	cmd.Stdout = &out
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vibes-anomaly: %w (stderr: %s)", err, errBuf.String())
	}

	var result rustOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse vibes-anomaly output: %w", err)
	}
	return result.Signals, nil
}
