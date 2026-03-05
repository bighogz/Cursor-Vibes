// Package rustclient invokes the Rust vibes-anomaly logic for anomaly detection
// and trend computation. It prefers in-process WebAssembly execution via wazero
// (zero subprocess overhead, single-binary deployment) and falls back to the
// native binary via os/exec if the .wasm module is not available.
package rustclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

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

// TrendResult matches the Rust QuarterlyTrend struct.
type TrendResult struct {
	QuarterPct float64 `json:"quarter_pct"`
	QReturn    float64 `json:"q_return"`
	Slope      float64 `json:"slope"`
	Last       float64 `json:"last"`
}

// --- Wasm runtime (preferred) ---

var (
	wasmOnce     sync.Once
	wasmRuntime  wazero.Runtime
	wasmCompiled wazero.CompiledModule
	wasmReady    bool
)

func initWasm() {
	wasmOnce.Do(func() {
		wasmPath := resolveWasmPath()
		if wasmPath == "" {
			return
		}
		data, err := os.ReadFile(wasmPath)
		if err != nil {
			return
		}
		ctx := context.Background()
		rt := wazero.NewRuntime(ctx)
		wasi_snapshot_preview1.MustInstantiate(ctx, rt)
		compiled, err := rt.CompileModule(ctx, data)
		if err != nil {
			log.Printf("rustclient: wasm compile error: %v", err)
			rt.Close(ctx)
			return
		}
		wasmRuntime = rt
		wasmCompiled = compiled
		wasmReady = true
		log.Printf("rustclient: loaded wasm module from %s (in-process, zero subprocess overhead)", wasmPath)
	})
}

func resolveWasmPath() string {
	if p := os.Getenv("VIBES_ANOMALY_WASM"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	cwd, _ := os.Getwd()
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	candidates := []string{
		filepath.Join(execDir, "vibes-anomaly.wasm"),
		filepath.Join(cwd, "bin", "vibes-anomaly.wasm"),
		filepath.Join(cwd, "rust-core", "target", "wasm32-wasip1", "release", "vibes-anomaly.wasm"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// runWasm executes the wasm module with the given subcommand and JSON stdin.
// Each invocation creates a fresh module instance (cheap — the compiled module
// is shared) so concurrent calls are safe without locking.
func runWasm(subcmd string, input interface{}) ([]byte, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	stdin := bytes.NewReader(body)
	stdout := &bytes.Buffer{}

	cfg := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(os.Stderr).
		WithArgs("vibes-anomaly", subcmd).
		WithName("")

	mod, err := wasmRuntime.InstantiateModule(ctx, wasmCompiled, cfg)
	if err != nil {
		return nil, fmt.Errorf("wasm instantiate: %w", err)
	}
	if mod != nil {
		mod.Close(ctx)
	}
	return stdout.Bytes(), nil
}

// --- Native binary fallback ---

var (
	binOnce sync.Once
	binPath string
)

func initBin() {
	binOnce.Do(func() {
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
			filepath.Join(execDir, "vibes-anomaly"),
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
	})
}

func validateBinPath(p string) bool {
	if p == "" || !filepath.IsAbs(p) {
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

func runBin(subcmd string, input interface{}) ([]byte, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binPath, subcmd)
	cmd.Stdin = bytes.NewReader(body)
	var out bytes.Buffer
	cmd.Stdout = &out
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vibes-anomaly %s: %w (stderr: %s)", subcmd, err, errBuf.String())
	}
	return out.Bytes(), nil
}

// --- Public API ---

// Available returns true if either wasm module or native binary is found.
func Available() bool {
	initWasm()
	if wasmReady {
		return true
	}
	initBin()
	return binPath != ""
}

// Mode returns "wasm", "subprocess", or "unavailable".
func Mode() string {
	initWasm()
	if wasmReady {
		return "wasm"
	}
	initBin()
	if binPath != "" {
		return "subprocess"
	}
	return "unavailable"
}

func run(subcmd string, input interface{}) ([]byte, error) {
	initWasm()
	if wasmReady {
		return runWasm(subcmd, input)
	}
	initBin()
	if binPath != "" {
		return runBin(subcmd, input)
	}
	return nil, fmt.Errorf("vibes-anomaly: neither wasm module nor native binary found")
}

// rustInput is sent to vibes-anomaly stdin.
type rustInput struct {
	Records []models.InsiderSellRecord `json:"records"`
	Params  struct {
		BaselineDays      int     `json:"baseline_days"`
		CurrentDays       int     `json:"current_days"`
		StdThreshold      float64 `json:"std_threshold"`
		MinBaselinePoints int     `json:"min_baseline_points"`
		AsOf              string  `json:"as_of"`
	} `json:"params"`
}

type rustOutput struct {
	Signals []AnomalySignal `json:"signals"`
}

// ComputeAnomalySignals runs vibes-anomaly anomaly with the given records and params.
func ComputeAnomalySignals(
	records []models.InsiderSellRecord,
	baselineDays, currentDays int,
	stdThreshold float64,
	asOf string,
) ([]AnomalySignal, error) {
	input := rustInput{Records: records}
	input.Params.BaselineDays = baselineDays
	input.Params.CurrentDays = currentDays
	input.Params.StdThreshold = stdThreshold
	input.Params.MinBaselinePoints = config.MinBaselinePoints
	input.Params.AsOf = asOf

	raw, err := run("anomaly", input)
	if err != nil {
		return nil, err
	}
	var result rustOutput
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse vibes-anomaly anomaly output: %w", err)
	}
	return result.Signals, nil
}

type trendInput struct {
	Closes []float64 `json:"closes"`
}

type trendOutput struct {
	Trend *TrendResult `json:"trend"`
}

// ComputeTrend runs vibes-anomaly trend to calculate quarterly return and slope.
func ComputeTrend(closes []float64) (*TrendResult, error) {
	raw, err := run("trend", trendInput{Closes: closes})
	if err != nil {
		return nil, err
	}
	var result trendOutput
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse vibes-anomaly trend output: %w", err)
	}
	return result.Trend, nil
}
