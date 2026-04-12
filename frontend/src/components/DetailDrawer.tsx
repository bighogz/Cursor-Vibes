import { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "../lib/cn";
import { fmtPrice, fmtPct, fmtShares, fmtValue } from "../lib/format";
import { PriceChart } from "./PriceChart";
import { IconX, IconExternalLink } from "./icons";
import type { AnomalyExplanation, Company, TrendKey } from "../types/dashboard";

const TREND_LABELS: Record<TrendKey, string> = {
  daily: "1 day",
  weekly: "1 week",
  monthly: "1 month",
  quarterly: "13 weeks",
};

interface Props {
  company: Company;
  trendPeriod?: TrendKey;
  onClose: () => void;
}

export function DetailDrawer({
  company: c,
  trendPeriod = "quarterly",
  onClose,
}: Props) {
  const drawerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    drawerRef.current?.focus();
  }, [c.symbol]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    }
    const el = drawerRef.current;
    el?.addEventListener("keydown", onKey);
    return () => el?.removeEventListener("keydown", onKey);
  }, [onClose]);

  const tp = c.trends?.[trendPeriod];
  const trendPct = tp?.pct ?? c.quarter_trend ?? null;
  const trendCloses = tp?.closes ?? c.quarter_closes ?? null;
  const isUp = (trendPct ?? 0) >= 0;
  const hasNews = c.news && c.news.length > 0;
  const hasInsiders = c.top_insiders && c.top_insiders.length > 0;
  const trendDesc = TREND_LABELS[trendPeriod] ?? "";

  const [explanation, setExplanation] = useState<AnomalyExplanation | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiError, setAiError] = useState<string | null>(null);

  useEffect(() => {
    setExplanation(null);
    setAiError(null);
    setAiLoading(false);
  }, [c.symbol]);

  const fetchExplanation = useCallback(async () => {
    setAiLoading(true);
    setAiError(null);
    try {
      const res = await fetch(`/api/ai/explain-anomaly?ticker=${encodeURIComponent(c.symbol)}`);
      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(body.detail ?? body.error ?? `HTTP ${res.status}`);
      }
      setExplanation(await res.json());
    } catch (err) {
      setAiError(err instanceof Error ? err.message : String(err));
    } finally {
      setAiLoading(false);
    }
  }, [c.symbol]);

  return (
    <div
      ref={drawerRef}
      tabIndex={-1}
      role="complementary"
      aria-label={`Details for ${c.symbol}`}
      className="w-[400px] flex-shrink-0 bg-surface-1 border-l border-line h-full
                 overflow-y-auto outline-none
                 animate-slide-in"
    >
      {/* Header */}
      <div className="sticky top-0 z-10 bg-surface-1 border-b border-line">
        <div className="flex items-center justify-between px-5 py-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-accent-hover font-semibold text-sm">
                {c.symbol}
              </span>
              {trendPct != null && (
                <span
                  className={cn(
                    "text-2xs font-medium tabular-nums",
                    isUp ? "text-positive" : "text-negative"
                  )}
                >
                  {fmtPct(trendPct)} {trendDesc}
                </span>
              )}
            </div>
            <p className="text-xs text-content-secondary truncate mt-0.5">
              {c.name}
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-md text-content-muted hover:text-content
                       hover:bg-surface-2 transition-colors"
            aria-label="Close detail panel"
          >
            <IconX size={16} />
          </button>
        </div>
      </div>

      <div className="px-5 py-4 space-y-6">
        {/* Price */}
        <Section title="Price">
          {c.price ? (
            <span className="text-2xl font-semibold tabular-nums text-content">
              {fmtPrice(c.price)}
            </span>
          ) : (
            <span className="text-sm text-content-muted">Not available</span>
          )}
        </Section>

        {/* Price Trend chart + period selector */}
        <Section title="Price Trend">
          {trendPct != null ? (
            <div>
              <div className="flex items-center gap-3 mb-3">
                <span
                  className={cn(
                    "text-lg font-semibold tabular-nums",
                    isUp ? "text-positive" : "text-negative"
                  )}
                >
                  {fmtPct(trendPct)}
                </span>
                <span className="text-2xs text-content-muted">{trendDesc}</span>
              </div>
              {trendCloses && trendCloses.length >= 2 && (
                <div className="bg-surface-0 rounded-lg p-3 border border-line">
                  <PriceChart
                    data={trendCloses}
                    positive={isUp}
                    width={340}
                    height={110}
                  />
                </div>
              )}
            </div>
          ) : (
            <span className="text-sm text-content-muted">Not available</span>
          )}
        </Section>

        {/* News */}
        <Section title="Recent News">
          {hasNews ? (
            <div className="space-y-2">
              {c.news!.slice(0, 3).map((n, i) => (
                <a
                  key={i}
                  href={n.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-start gap-2 group/link py-1.5"
                >
                  <span className="text-xs text-content-secondary group-hover/link:text-accent-hover transition-colors flex-1 leading-relaxed">
                    {n.title}
                  </span>
                  <IconExternalLink
                    size={12}
                    className="text-content-muted flex-shrink-0 mt-0.5 opacity-0 group-hover/link:opacity-100 transition-opacity"
                  />
                </a>
              ))}
            </div>
          ) : (
            <span className="text-sm text-content-muted">
              No recent news available
            </span>
          )}
        </Section>

        {/* Top insider sellers */}
        <Section title="Top Insider Sellers">
          {hasInsiders ? (
            <div className="border border-line rounded-lg overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="bg-surface-0 border-b border-line">
                    <th className="text-left px-3 py-1.5 text-2xs font-medium text-content-muted uppercase tracking-wider">
                      Type
                    </th>
                    <th className="text-left px-3 py-1.5 text-2xs font-medium text-content-muted uppercase tracking-wider">
                      Name
                    </th>
                    <th className="text-right px-3 py-1.5 text-2xs font-medium text-content-muted uppercase tracking-wider">
                      Shares
                    </th>
                    <th className="text-right px-3 py-1.5 text-2xs font-medium text-content-muted uppercase tracking-wider">
                      Value
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {c.top_insiders!.map((ins, i) => (
                    <tr
                      key={i}
                      className="border-b border-line last:border-b-0"
                    >
                      <td className="px-3 py-2 text-content-muted text-2xs">
                        {ins.tx_type ?? "—"}
                      </td>
                      <td className="px-3 py-2 text-content-secondary">
                        <div className="truncate max-w-[140px]" title={ins.name}>
                          {ins.name}
                        </div>
                        {ins.role && (
                          <div className="text-2xs text-content-muted">
                            {ins.role}
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums text-content-secondary">
                        {fmtShares(ins.shares)}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums text-content-secondary">
                        {fmtValue(ins.value)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <span className="text-sm text-content-muted">
              No insider data available
            </span>
          )}
        </Section>

        {/* AI Anomaly Explanation */}
        {explanation ? (
          <Section title="AI Anomaly Explanation">
            <div className="space-y-3">
              <p className="text-xs text-content-secondary leading-relaxed">
                {explanation.summary}
              </p>
              {explanation.drivers.length > 0 && (
                <div>
                  <h4 className="text-2xs font-medium text-content-muted mb-1">
                    Drivers
                  </h4>
                  <ul className="space-y-0.5">
                    {explanation.drivers.map((d, i) => (
                      <li
                        key={i}
                        className="text-xs text-content-secondary flex gap-1.5"
                      >
                        <span className="text-content-muted flex-shrink-0">•</span>
                        {d}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {explanation.caveats.length > 0 && (
                <div>
                  <h4 className="text-2xs font-medium text-content-muted mb-1">
                    Caveats
                  </h4>
                  <ul className="space-y-0.5">
                    {explanation.caveats.map((cv, i) => (
                      <li
                        key={i}
                        className="text-xs text-content-secondary flex gap-1.5"
                      >
                        <span className="text-content-muted flex-shrink-0">•</span>
                        {cv}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <button
                onClick={fetchExplanation}
                disabled={aiLoading}
                className="text-2xs text-content-muted hover:text-content transition-colors"
              >
                Re-run
              </button>
            </div>
          </Section>
        ) : (
          <div>
            {aiError && (
              <p className="text-2xs text-negative mb-1.5">{aiError}</p>
            )}
            <button
              onClick={fetchExplanation}
              disabled={aiLoading}
              className={cn(
                "text-xs px-3 py-1.5 rounded-md border transition-colors",
                aiLoading
                  ? "border-line text-content-muted cursor-wait"
                  : "border-accent text-accent hover:bg-accent hover:text-white"
              )}
            >
              {aiLoading ? "Analyzing…" : "Explain Anomaly"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h3 className="text-2xs font-medium uppercase tracking-wider text-content-muted mb-2">
        {title}
      </h3>
      {children}
    </div>
  );
}
