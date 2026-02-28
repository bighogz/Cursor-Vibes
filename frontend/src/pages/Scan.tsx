import { useState } from "react";
import { cn } from "../lib/cn";
import { fetchScan } from "../lib/api";
import type { ScanResult, ScanSignal } from "../types/dashboard";
import { IconAlert } from "../components/icons";

export function Scan() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ScanResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showAll, setShowAll] = useState(false);

  const [params, setParams] = useState({
    baseline_days: 365,
    current_days: 30,
    std_threshold: 2.0,
    limit: 25,
  });

  async function runScan() {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchScan(params);
      if (data.error) setError(data.error);
      setResult(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Scan failed");
    } finally {
      setLoading(false);
    }
  }

  const signals: ScanSignal[] = showAll
    ? result?.all_signals ?? []
    : result?.anomalies ?? [];

  return (
    <div className="max-w-5xl mx-auto px-5 py-6">
      <div className="flex items-center gap-3 mb-6">
        <IconAlert size={20} className="text-accent" />
        <h2 className="text-base font-semibold text-content">
          Anomaly Detection
        </h2>
      </div>

      {/* Parameters */}
      <div className="bg-surface-1 border border-line rounded-lg p-4 mb-6">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Field label="Baseline days">
            <input
              type="number"
              value={params.baseline_days}
              onChange={(e) =>
                setParams((p) => ({
                  ...p,
                  baseline_days: Number(e.target.value),
                }))
              }
              className="field-input"
              min={30}
              max={730}
            />
          </Field>
          <Field label="Current window">
            <input
              type="number"
              value={params.current_days}
              onChange={(e) =>
                setParams((p) => ({
                  ...p,
                  current_days: Number(e.target.value),
                }))
              }
              className="field-input"
              min={7}
              max={90}
            />
          </Field>
          <Field label="Z-score threshold">
            <input
              type="number"
              value={params.std_threshold}
              onChange={(e) =>
                setParams((p) => ({
                  ...p,
                  std_threshold: Number(e.target.value),
                }))
              }
              className="field-input"
              step={0.1}
              min={1}
              max={5}
            />
          </Field>
          <Field label="Ticker limit">
            <input
              type="number"
              value={params.limit}
              onChange={(e) =>
                setParams((p) => ({ ...p, limit: Number(e.target.value) }))
              }
              className="field-input"
              min={5}
              max={503}
            />
          </Field>
        </div>
        <button
          onClick={runScan}
          disabled={loading}
          className="mt-4 px-4 py-2 bg-accent hover:bg-accent-hover text-white text-[13px]
                     font-medium rounded-md transition-colors disabled:opacity-50"
        >
          {loading ? "Scanning…" : "Run Scan"}
        </button>
      </div>

      {error && (
        <div className="px-4 py-3 rounded-lg bg-negative-dim border border-negative/20 text-negative text-[13px] mb-4">
          {error}
        </div>
      )}

      {result && (
        <>
          {/* Summary */}
          <div className="flex items-center gap-6 mb-4 text-xs text-content-secondary">
            <span>
              <strong className="text-content">{result.tickers_count}</strong>{" "}
              tickers scanned
            </span>
            <span>
              <strong className="text-content">{result.records_count}</strong>{" "}
              records
            </span>
            <span>
              <strong className="text-negative">{result.anomalies_count}</strong>{" "}
              anomalies
            </span>
            <span>
              {result.date_from} → {result.date_to}
            </span>
          </div>

          {/* Toggle */}
          <div className="flex gap-2 mb-3">
            <button
              onClick={() => setShowAll(false)}
              className={cn(
                "px-3 py-1 rounded-md text-xs font-medium transition-colors",
                !showAll
                  ? "bg-accent-dim text-accent-hover"
                  : "text-content-muted hover:text-content"
              )}
            >
              Anomalies ({result.anomalies_count})
            </button>
            <button
              onClick={() => setShowAll(true)}
              className={cn(
                "px-3 py-1 rounded-md text-xs font-medium transition-colors",
                showAll
                  ? "bg-accent-dim text-accent-hover"
                  : "text-content-muted hover:text-content"
              )}
            >
              All signals ({result.all_signals?.length ?? 0})
            </button>
          </div>

          {/* Results table */}
          <div className="border border-line rounded-lg overflow-hidden">
            <table className="w-full text-[13px] border-collapse">
              <thead>
                <tr className="bg-surface-1 border-b border-line">
                  <th className="text-left px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Ticker
                  </th>
                  <th className="text-right px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Current Selling
                  </th>
                  <th className="text-right px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Baseline Mean
                  </th>
                  <th className="text-right px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Baseline Std
                  </th>
                  <th className="text-right px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Z-Score
                  </th>
                  <th className="text-center px-4 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody>
                {signals.map((s) => (
                  <tr
                    key={s.ticker}
                    className="border-b border-line last:border-b-0 hover:bg-surface-2/50"
                  >
                    <td className="px-4 py-2.5 font-medium text-accent-hover">
                      {s.ticker}
                    </td>
                    <td className="px-4 py-2.5 text-right tabular-nums text-content-secondary">
                      {Math.round(s.current_shares_sold).toLocaleString()}
                    </td>
                    <td className="px-4 py-2.5 text-right tabular-nums text-content-secondary">
                      {s.baseline_mean.toFixed(1)}
                    </td>
                    <td className="px-4 py-2.5 text-right tabular-nums text-content-secondary">
                      {s.baseline_std.toFixed(1)}
                    </td>
                    <td className="px-4 py-2.5 text-right tabular-nums font-medium">
                      <span
                        className={cn(
                          s.z_score >= 2 ? "text-negative" : "text-content"
                        )}
                      >
                        {s.z_score.toFixed(2)}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-center">
                      {s.is_anomaly ? (
                        <span className="inline-flex px-2 py-0.5 rounded text-2xs font-medium bg-negative-dim text-negative">
                          Anomaly
                        </span>
                      ) : (
                        <span className="text-2xs text-content-muted">
                          Normal
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
                {signals.length === 0 && (
                  <tr>
                    <td
                      colSpan={6}
                      className="px-4 py-12 text-center text-content-muted text-sm"
                    >
                      {loading ? "Running scan…" : "No signals to display"}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-2xs font-medium text-content-muted mb-1 uppercase tracking-wider">
        {label}
      </label>
      {children}
    </div>
  );
}
