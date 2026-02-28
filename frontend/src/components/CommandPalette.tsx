import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "../lib/cn";
import { useToast } from "../context/toast-context";
import type { Company } from "../types/dashboard";
import {
  IconDashboard,
  IconAlert,
  IconSettings,
  IconRefresh,
  IconCopy,
  IconSearch,
  IconTrending,
} from "./icons";

interface ActionItem {
  id: string;
  label: string;
  section: string;
  icon: React.ReactNode;
  onSelect: () => void;
  shortcut?: string;
}

interface Props {
  open: boolean;
  onClose: () => void;
  companies: Company[];
  onSelectStock: (sym: string) => void;
  onRefresh: () => void;
}

export function CommandPalette({
  open,
  onClose,
  companies,
  onSelectStock,
  onRefresh,
}: Props) {
  const [query, setQuery] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const { toast } = useToast();

  const staticActions: ActionItem[] = useMemo(
    () => [
      {
        id: "nav-dashboard",
        label: "Go to Dashboard",
        section: "Navigation",
        icon: <IconDashboard size={16} />,
        onSelect: () => navigate("/"),
      },
      {
        id: "nav-scan",
        label: "Go to Anomaly Scan",
        section: "Navigation",
        icon: <IconAlert size={16} />,
        onSelect: () => navigate("/scan"),
      },
      {
        id: "nav-settings",
        label: "Go to Settings",
        section: "Navigation",
        icon: <IconSettings size={16} />,
        onSelect: () => navigate("/settings"),
      },
      {
        id: "action-refresh",
        label: "Refresh Dashboard Data",
        section: "Actions",
        icon: <IconRefresh size={16} />,
        onSelect: () => {
          onRefresh();
          toast("Dashboard refresh started", "success");
        },
      },
      {
        id: "action-copy",
        label: "Copy Current URL",
        section: "Actions",
        icon: <IconCopy size={16} />,
        onSelect: () => {
          navigator.clipboard.writeText(window.location.href);
          toast("URL copied to clipboard", "success");
        },
      },
    ],
    [navigate, onRefresh, toast]
  );

  const stockResults: ActionItem[] = useMemo(() => {
    if (!query.trim()) return [];
    const q = query.toLowerCase();
    return companies
      .filter(
        (c) =>
          c.symbol.toLowerCase().includes(q) ||
          c.name.toLowerCase().includes(q)
      )
      .slice(0, 12)
      .map((c) => ({
        id: `stock-${c.symbol}`,
        label: `${c.symbol} — ${c.name}`,
        section: "Stocks",
        icon: <IconTrending size={16} />,
        onSelect: () => onSelectStock(c.symbol),
      }));
  }, [query, companies, onSelectStock]);

  const filteredStatic = useMemo(() => {
    if (!query.trim()) return staticActions;
    const q = query.toLowerCase();
    return staticActions.filter((a) => a.label.toLowerCase().includes(q));
  }, [query, staticActions]);

  const allItems = useMemo(
    () => [...stockResults, ...filteredStatic],
    [stockResults, filteredStatic]
  );

  // Reset state when opened
  useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIdx(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  // Clamp active index
  useEffect(() => {
    setActiveIdx((i) => Math.min(i, Math.max(0, allItems.length - 1)));
  }, [allItems.length]);

  const selectItem = useCallback(
    (idx: number) => {
      allItems[idx]?.onSelect();
      onClose();
    },
    [allItems, onClose]
  );

  // Scroll active item into view
  useEffect(() => {
    const el = listRef.current?.querySelector(`[data-idx="${activeIdx}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [activeIdx]);

  function handleKeyDown(e: React.KeyboardEvent) {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setActiveIdx((i) => Math.min(i + 1, allItems.length - 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setActiveIdx((i) => Math.max(i - 1, 0));
        break;
      case "Enter":
        e.preventDefault();
        selectItem(activeIdx);
        break;
      case "Escape":
        e.preventDefault();
        onClose();
        break;
    }
  }

  if (!open) return null;

  // Group items by section
  const sections = new Map<string, { idx: number; item: ActionItem }[]>();
  allItems.forEach((item, idx) => {
    const list = sections.get(item.section) ?? [];
    list.push({ idx, item });
    sections.set(item.section, list);
  });

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Dialog */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh]"
        onKeyDown={handleKeyDown}
      >
        <div className="w-full max-w-lg bg-surface-1 border border-line rounded-xl shadow-2xl shadow-black/40 overflow-hidden">
          {/* Search input */}
          <div className="flex items-center gap-3 px-4 border-b border-line">
            <IconSearch size={16} className="text-content-muted flex-shrink-0" />
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Type a command or search…"
              className="flex-1 bg-transparent py-3 text-[13px] text-content
                         placeholder:text-content-muted outline-none"
              autoComplete="off"
              spellCheck={false}
            />
            <kbd className="text-2xs text-content-muted bg-surface-2 px-1.5 py-0.5 rounded border border-line">
              ESC
            </kbd>
          </div>

          {/* Results */}
          <div ref={listRef} className="max-h-[320px] overflow-y-auto py-1.5">
            {allItems.length === 0 && (
              <div className="px-4 py-8 text-center text-xs text-content-muted">
                No results found
              </div>
            )}

            {Array.from(sections.entries()).map(([section, items]) => (
              <div key={section}>
                <div className="px-4 py-1.5 text-2xs font-medium uppercase tracking-wider text-content-muted">
                  {section}
                </div>
                {items.map(({ idx, item }) => (
                  <button
                    key={item.id}
                    data-idx={idx}
                    onClick={() => selectItem(idx)}
                    onMouseEnter={() => setActiveIdx(idx)}
                    className={cn(
                      "w-full flex items-center gap-3 px-4 py-2 text-[13px] text-left transition-colors",
                      idx === activeIdx
                        ? "bg-accent-dim text-content"
                        : "text-content-secondary hover:text-content"
                    )}
                  >
                    <span className="flex-shrink-0 text-content-muted">
                      {item.icon}
                    </span>
                    <span className="flex-1 truncate">{item.label}</span>
                    {item.shortcut && (
                      <kbd className="text-2xs text-content-muted">
                        {item.shortcut}
                      </kbd>
                    )}
                  </button>
                ))}
              </div>
            ))}
          </div>

          {/* Footer hints */}
          <div className="flex items-center gap-4 px-4 py-2 border-t border-line text-2xs text-content-muted">
            <span>
              <kbd className="bg-surface-2 px-1 rounded border border-line">
                ↑↓
              </kbd>{" "}
              navigate
            </span>
            <span>
              <kbd className="bg-surface-2 px-1 rounded border border-line">
                ↵
              </kbd>{" "}
              select
            </span>
            <span>
              <kbd className="bg-surface-2 px-1 rounded border border-line">
                esc
              </kbd>{" "}
              close
            </span>
          </div>
        </div>
      </div>
    </>
  );
}
