import { useState, type ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { cn } from "../lib/cn";
import {
  IconDashboard,
  IconAlert,
  IconSettings,
  IconChevronDown,
  IconCommand,
} from "./icons";

interface NavItem {
  to: string;
  label: string;
  icon: ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  { to: "/", label: "Dashboard", icon: <IconDashboard size={16} /> },
  { to: "/scan", label: "Anomaly Scan", icon: <IconAlert size={16} /> },
];

interface Props {
  sectors: string[];
  activeSector: string;
  onSectorChange: (s: string) => void;
  onOpenCommandPalette: () => void;
}

export function SidebarNav({
  sectors,
  activeSector,
  onSectorChange,
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
            <span className="text-white text-2xs font-semibold">V</span>
          </div>
          <span className="text-[13px] font-semibold text-content">Vibes</span>
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
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] transition-colors",
                  isActive
                    ? "bg-accent-dim text-accent-hover font-medium"
                    : "text-content-secondary hover:text-content hover:bg-surface-2"
                )
              }
            >
              {item.icon}
              {item.label}
            </NavLink>
          ))}
        </div>

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
