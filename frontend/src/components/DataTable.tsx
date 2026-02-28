import { memo, useCallback, useEffect, useRef, useState } from "react";
import { cn } from "../lib/cn";
import { fmtPrice, fmtPct, fmtShares } from "../lib/format";
import { Sparkline } from "./Sparkline";
import type { Company, Sector } from "../types/dashboard";

const COL_COUNT = 7;

interface Props {
  sectors: Sector[];
  selectedStock: string;
  onSelectStock: (sym: string) => void;
  loading: boolean;
}

interface FlatRow {
  type: "header" | "company";
  sectorName?: string;
  sectorCount?: number;
  company?: Company;
}

function flatten(sectors: Sector[]): FlatRow[] {
  const rows: FlatRow[] = [];
  for (const s of sectors) {
    rows.push({
      type: "header",
      sectorName: s.name,
      sectorCount: s.companies.length,
    });
    for (const c of s.companies) {
      rows.push({ type: "company", company: c });
    }
  }
  return rows;
}

export function DataTable({
  sectors,
  selectedStock,
  onSelectStock,
  loading,
}: Props) {
  const flat = flatten(sectors);
  const companyIndices = flat
    .map((r, i) => (r.type === "company" ? i : -1))
    .filter((i) => i >= 0);

  const [focusedIdx, setFocusedIdx] = useState(-1);
  const tableRef = useRef<HTMLDivElement>(null);

  const focusedRow = focusedIdx >= 0 ? flat[companyIndices[focusedIdx]] : null;

  // Keyboard navigation
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      )
        return;
      // Don't handle if command palette or drawer has focus
      if (target.closest("[role='dialog']")) return;

      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setFocusedIdx((i) => Math.min(i + 1, companyIndices.length - 1));
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setFocusedIdx((i) => Math.max(i - 1, 0));
      } else if (e.key === "Enter" && focusedRow?.company) {
        e.preventDefault();
        onSelectStock(focusedRow.company.symbol);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [companyIndices.length, focusedRow, onSelectStock]);

  // Scroll focused row into view
  useEffect(() => {
    if (focusedIdx < 0) return;
    const flatIdx = companyIndices[focusedIdx];
    const el = tableRef.current?.querySelector(`[data-row="${flatIdx}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [focusedIdx, companyIndices]);

  if (loading && sectors.length === 0) {
    return <SkeletonTable />;
  }

  if (sectors.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-content-muted text-sm">
        Dashboard is building. This usually takes 2–3 minutes on first run.
      </div>
    );
  }

  return (
    <div ref={tableRef} className="w-full">
      <table className="w-full border-collapse text-[13px]">
        <thead className="sticky top-0 z-10 bg-surface-0">
          <tr className="border-b border-line">
            <Th className="w-[72px] pl-5">Symbol</Th>
            <Th className="w-[14%]">Company</Th>
            <Th className="w-[80px] text-right">Price</Th>
            <Th className="w-[72px] text-right">Change</Th>
            <Th className="w-[180px]">Quarterly Trend</Th>
            <Th className="w-[180px]">Recent News</Th>
            <Th className="min-w-[150px]">Top Insiders</Th>
          </tr>
        </thead>
        <tbody>
          {flat.map((row, i) =>
            row.type === "header" ? (
              <SectorHeader
                key={`h-${row.sectorName}`}
                name={row.sectorName!}
                count={row.sectorCount!}
              />
            ) : (
              <CompanyRow
                key={row.company!.symbol}
                company={row.company!}
                dataRow={i}
                selected={row.company!.symbol === selectedStock}
                focused={companyIndices[focusedIdx] === i}
                onClick={() => onSelectStock(row.company!.symbol)}
              />
            )
          )}
        </tbody>
      </table>
    </div>
  );
}

function Th({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <th
      className={cn(
        "px-3 py-2 text-2xs font-medium uppercase tracking-wider text-content-muted text-left",
        className
      )}
    >
      {children}
    </th>
  );
}

const SectorHeader = memo(function SectorHeader({
  name,
  count,
}: {
  name: string;
  count: number;
}) {
  return (
    <tr className="bg-surface-1 border-y border-line">
      <td colSpan={COL_COUNT} className="px-5 py-2">
        <div className="flex items-center gap-2.5">
          <span className="text-xs font-medium text-content">{name}</span>
          <span className="text-2xs text-content-muted tabular-nums bg-surface-0 px-1.5 py-0.5 rounded">
            {count}
          </span>
        </div>
      </td>
    </tr>
  );
});

const CompanyRow = memo(function CompanyRow({
  company: c,
  dataRow,
  selected,
  focused,
  onClick,
}: {
  company: Company;
  dataRow: number;
  selected: boolean;
  focused: boolean;
  onClick: () => void;
}) {
  const handleClick = useCallback(() => onClick(), [onClick]);
  const isUp = (c.quarter_trend ?? 0) >= 0;
  const hasNews = c.news && c.news.length > 0;
  const hasInsiders = c.top_insiders && c.top_insiders.length > 0;

  return (
    <tr
      data-row={dataRow}
      onClick={handleClick}
      className={cn(
        "border-b border-line cursor-pointer transition-colors group",
        selected && "bg-accent-dim",
        focused && !selected && "bg-surface-2",
        !selected && !focused && "hover:bg-surface-2/50"
      )}
    >
      {/* Symbol */}
      <td className="pl-5 pr-3 py-2.5">
        <div className="flex items-center gap-1.5">
          {selected && (
            <div className="w-0.5 h-4 rounded-full bg-accent -ml-3 mr-1.5" />
          )}
          <span className="text-accent-hover font-medium text-[13px]">
            {c.symbol}
          </span>
        </div>
      </td>

      {/* Name */}
      <td className="px-3 py-2.5">
        <span className="text-content-secondary truncate block max-w-[200px]" title={c.name}>
          {c.name}
        </span>
      </td>

      {/* Price */}
      <td className="px-3 py-2.5 text-right tabular-nums font-medium">
        {c.price ? fmtPrice(c.price) : <Muted />}
      </td>

      {/* Change */}
      <td className="px-3 py-2.5 text-right">
        {c.change_pct != null ? (
          <span
            className={cn(
              "inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium tabular-nums",
              c.change_pct >= 0
                ? "bg-positive-dim text-positive"
                : "bg-negative-dim text-negative"
            )}
          >
            {fmtPct(c.change_pct)}
          </span>
        ) : (
          <Muted />
        )}
      </td>

      {/* Quarterly trend */}
      <td className="px-3 py-2.5">
        {c.quarter_trend != null ? (
          <div className="flex items-center gap-2">
            {c.quarter_closes && c.quarter_closes.length >= 2 && (
              <Sparkline data={c.quarter_closes} positive={isUp} />
            )}
            <span
              className={cn(
                "text-xs font-medium tabular-nums",
                isUp ? "text-positive" : "text-negative"
              )}
            >
              {fmtPct(c.quarter_trend)}
            </span>
          </div>
        ) : (
          <Muted />
        )}
      </td>

      {/* News */}
      <td className="px-3 py-2.5">
        {hasNews ? (
          <a
            href={c.news![0].url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="text-xs text-content-secondary hover:text-accent-hover truncate block max-w-[240px] transition-colors"
            title={c.news![0].title}
          >
            {c.news![0].title}
          </a>
        ) : (
          <Muted />
        )}
      </td>

      {/* Top Insiders */}
      <td className="px-3 py-2.5">
        {hasInsiders ? (
          <div className="space-y-0.5">
            {c.top_insiders!.slice(0, 2).map((ins, idx) => (
              <div
                key={idx}
                className="flex items-center gap-1.5 text-xs text-content-secondary"
              >
                <span className="truncate max-w-[100px]" title={ins.name}>
                  {ins.name}
                </span>
                <span className="text-content-muted tabular-nums flex-shrink-0">
                  {fmtShares(ins.shares)}
                </span>
              </div>
            ))}
            {c.top_insiders!.length > 2 && (
              <span className="text-2xs text-content-muted">
                +{c.top_insiders!.length - 2} more
              </span>
            )}
          </div>
        ) : (
          <Muted />
        )}
      </td>
    </tr>
  );
});

function Muted() {
  return <span className="text-content-muted">—</span>;
}

function SkeletonTable() {
  return (
    <div className="w-full">
      <table className="w-full border-collapse text-[13px]">
        <thead className="sticky top-0 z-10 bg-surface-0">
          <tr className="border-b border-line">
            <th className="px-5 py-2 w-[72px]" />
            <th className="px-3 py-2 w-[14%]" />
            <th className="px-3 py-2 w-[80px]" />
            <th className="px-3 py-2 w-[72px]" />
            <th className="px-3 py-2 w-[180px]" />
            <th className="px-3 py-2 w-[180px]" />
            <th className="px-3 py-2" />
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 20 }).map((_, i) => (
            <tr key={i} className="border-b border-line">
              <td className="pl-5 pr-3 py-3">
                <div className="h-3.5 w-10 bg-surface-2 rounded animate-pulse" />
              </td>
              <td className="px-3 py-3">
                <div className="h-3.5 w-32 bg-surface-2 rounded animate-pulse" />
              </td>
              <td className="px-3 py-3">
                <div className="h-3.5 w-14 bg-surface-2 rounded animate-pulse ml-auto" />
              </td>
              <td className="px-3 py-3">
                <div className="h-3.5 w-12 bg-surface-2 rounded animate-pulse ml-auto" />
              </td>
              <td className="px-3 py-3">
                <div className="h-6 w-24 bg-surface-2 rounded animate-pulse" />
              </td>
              <td className="px-3 py-3">
                <div className="h-3.5 w-40 bg-surface-2 rounded animate-pulse" />
              </td>
              <td className="px-3 py-3">
                <div className="h-3.5 w-28 bg-surface-2 rounded animate-pulse" />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
