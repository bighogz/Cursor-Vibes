import { useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { cn } from "../lib/cn";
import {
  IconDashboard,
  IconSettings,
  IconChevronDown,
  IconCommand,
} from "./icons";
import type { TrendKey } from "../types/dashboard";

const TREND_OPTIONS: { key: TrendKey; label: string }[] = [
  { key: "daily", label: "1D" },
  { key: "weekly", label: "1W" },
  { key: "monthly", label: "1M" },
  { key: "quarterly", label: "3M" },
];

interface Props {
  sectors: string[];
  activeSector: string;
  onSectorChange: (s: string) => void;
  activeTrend: TrendKey;
  onTrendChange: (t: TrendKey) => void;
  onOpenCommandPalette: () => void;
}

export function SidebarNav({
  sectors,
  activeSector,
  onSectorChange,
  activeTrend,
  onTrendChange,
  onOpenCommandPalette,
}: Props) {
  const [sectorsOpen, setSectorsOpen] = useState(true);
  const location = useLocation();
  const isDashboard = location.pathname === "/";

  return (
    <aside className="w-[220px] flex-shrink-0 h-screen bg-surface-1 border-r border-line flex flex-col">
      {/* Logo */}
      <div className="h-12 flex items-center px-4 border-b border-line">
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 rounded bg-accent flex items-center justify-center">
            <span className="text-white text-2xs font-bold">5</span>
          </div>
          <span className="text-[13px] font-semibold text-content">500-sketchpad</span>
        </div>
      </div>

      {/* Search trigger */}
      <button
        onClick={onOpenCommandPalette}
        className="mx-3 mt-3 mb-1 flex items-center gap-2 px-2.5 py-1.5 rounded-md
                   bg-surface-2 border border-line text-content-muted text-xs
                   hover:border-line-strong hover:text-content-secondary transition-colors"
      >
        <IconCommand size={13} />
        <span className="flex-1 text-left">Search…</span>
        <kbd className="text-2xs text-content-muted bg-surface-1 px-1 rounded border border-line">
          ⌘K
        </kbd>
      </button>

      {/* Primary nav */}
      <nav className="mt-2 px-2 flex-1 overflow-y-auto" aria-label="Main">
        <div className="space-y-0.5">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] transition-colors",
                isActive
                  ? "bg-accent-dim text-accent-hover font-medium"
                  : "text-content-secondary hover:text-content hover:bg-surface-2"
              )
            }
          >
            <IconDashboard size={16} />
            Dashboard
          </NavLink>
        </div>

        {/* Trend period (only on dashboard) */}
        {isDashboard && (
          <div className="mt-5">
            <span className="flex items-center gap-1 px-2.5 w-full text-2xs font-medium uppercase tracking-wider text-content-muted">
              Trend
            </span>
            <div className="mt-1 flex gap-1 px-2.5">
              {TREND_OPTIONS.map((opt) => (
                <button
                  key={opt.key}
                  onClick={() => onTrendChange(opt.key)}
                  className={cn(
                    "flex-1 py-1 rounded-md text-xs font-medium text-center transition-colors",
                    activeTrend === opt.key
                      ? "bg-accent-dim text-accent-hover"
                      : "text-content-muted hover:text-content hover:bg-surface-2"
                  )}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Sector filter (only on dashboard) */}
        {isDashboard && sectors.length > 0 && (
          <div className="mt-5">
            <button
              onClick={() => setSectorsOpen(!sectorsOpen)}
              className="flex items-center gap-1 px-2.5 w-full text-2xs font-medium
                         uppercase tracking-wider text-content-muted hover:text-content-secondary transition-colors"
            >
              <IconChevronDown
                size={12}
                className={cn(
                  "transition-transform",
                  !sectorsOpen && "-rotate-90"
                )}
              />
              Sectors
            </button>

            {sectorsOpen && (
              <div className="mt-1 space-y-0.5">
                <button
                  onClick={() => onSectorChange("")}
                  className={cn(
                    "w-full text-left px-2.5 py-1 rounded-md text-xs transition-colors",
                    activeSector === ""
                      ? "bg-accent-dim text-accent-hover"
                      : "text-content-secondary hover:text-content hover:bg-surface-2"
                  )}
                >
                  All sectors
                </button>
                {sectors.map((s) => (
                  <button
                    key={s}
                    onClick={() => onSectorChange(s)}
                    className={cn(
                      "w-full text-left px-2.5 py-1 rounded-md text-xs truncate transition-colors",
                      activeSector === s
                        ? "bg-accent-dim text-accent-hover"
                        : "text-content-secondary hover:text-content hover:bg-surface-2"
                    )}
                    title={s}
                  >
                    {s}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
      </nav>

      {/* Bottom */}
      <div className="px-2 pb-3 border-t border-line pt-2">
        <NavLink
          to="/settings"
          className={({ isActive }) =>
            cn(
              "flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] transition-colors",
              isActive
                ? "bg-accent-dim text-accent-hover font-medium"
                : "text-content-secondary hover:text-content hover:bg-surface-2"
            )
          }
        >
          <IconSettings size={16} />
          Settings
        </NavLink>
      </div>
    </aside>
  );
}
