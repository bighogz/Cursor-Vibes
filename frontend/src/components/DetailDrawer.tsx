import { useEffect, useRef } from "react";
import { cn } from "../lib/cn";
import { fmtPrice, fmtPct, fmtShares, fmtValue } from "../lib/format";
import { Sparkline } from "./Sparkline";
import { IconX, IconExternalLink } from "./icons";
import type { Company } from "../types/dashboard";

interface Props {
  company: Company;
  onClose: () => void;
}

export function DetailDrawer({ company: c, onClose }: Props) {
  const drawerRef = useRef<HTMLDivElement>(null);

  // Focus trap: focus the drawer on mount
  useEffect(() => {
    drawerRef.current?.focus();
  }, [c.symbol]);

  // Close on Escape
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

  const isUp = (c.quarter_trend ?? 0) >= 0;
  const hasNews = c.news && c.news.length > 0;
  const hasInsiders = c.top_insiders && c.top_insiders.length > 0;

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
              {c.change_pct != null && (
                <span
                  className={cn(
                    "inline-flex items-center px-1.5 py-0.5 rounded text-2xs font-medium tabular-nums",
                    (c.change_pct ?? 0) >= 0
                      ? "bg-positive-dim text-positive"
                      : "bg-negative-dim text-negative"
                  )}
                >
                  {fmtPct(c.change_pct)}
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
            <div className="flex items-baseline gap-3">
              <span className="text-2xl font-semibold tabular-nums text-content">
                {fmtPrice(c.price)}
              </span>
              {c.sources?.price && (
                <span className="text-2xs text-content-muted">
                  via {c.sources.price}
                </span>
              )}
            </div>
          ) : (
            <span className="text-sm text-content-muted">Not available</span>
          )}
        </Section>

        {/* Quarterly trend */}
        <Section title="Quarterly Trend">
          {c.quarter_trend != null ? (
            <div>
              <div className="flex items-center gap-3 mb-2">
                <span
                  className={cn(
                    "text-lg font-semibold tabular-nums",
                    isUp ? "text-positive" : "text-negative"
                  )}
                >
                  {fmtPct(c.quarter_trend)}
                </span>
                <span className="text-2xs text-content-muted">13 weeks</span>
              </div>
              {c.quarter_closes && c.quarter_closes.length >= 2 && (
                <div className="bg-surface-0 rounded-lg p-3 border border-line">
                  <Sparkline
                    data={c.quarter_closes}
                    positive={isUp}
                    width={340}
                    height={80}
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
              {c.news!.map((n, i) => (
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
                      <td className="px-3 py-2 text-content-secondary">
                        <div className="truncate max-w-[160px]" title={ins.name}>
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

        {/* Sources */}
        {c.sources && (
          <Section title="Data Sources">
            <div className="flex flex-wrap gap-2">
              {Object.entries(c.sources).map(([k, v]) => (
                <span
                  key={k}
                  className="text-2xs bg-surface-0 border border-line rounded px-2 py-1 text-content-muted"
                >
                  {k}: <span className="text-content-secondary">{v}</span>
                </span>
              ))}
            </div>
          </Section>
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
