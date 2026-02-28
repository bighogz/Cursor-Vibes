import { useCallback, useEffect, useState } from "react";
import { Outlet, useLocation, useSearchParams } from "react-router-dom";
import { SidebarNav } from "./SidebarNav";
import { CommandPalette } from "./CommandPalette";
import { DetailDrawer } from "./DetailDrawer";
import { ToastContainer } from "./Toast";
import { fetchDashboard } from "../lib/api";
import type { Company, DashboardData } from "../types/dashboard";

export function AppShell() {
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [cmdOpen, setCmdOpen] = useState(false);

  const sector = searchParams.get("sector") ?? "";
  const selectedStock = searchParams.get("stock") ?? "";

  const load = useCallback(
    async (s?: string) => {
      setLoading(true);
      setError(null);
      try {
        const sectorParam = s !== undefined ? s : sector;
        const d = await fetchDashboard(sectorParam || undefined, 50);
        if (d.error) {
          setError(d.error);
        }
        setData(d);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load dashboard");
      } finally {
        setLoading(false);
      }
    },
    [sector]
  );

  useEffect(() => {
    if (location.pathname === "/" || location.pathname === "") {
      load();
    }
  }, [location.pathname, load]);

  const allCompanies: Company[] =
    data?.sectors?.flatMap((s) => s.companies) ?? [];

  const selectedCompany = selectedStock
    ? allCompanies.find((c) => c.symbol === selectedStock) ?? null
    : null;

  const handleSectorChange = (s: string) => {
    const next = new URLSearchParams(searchParams);
    if (s) {
      next.set("sector", s);
    } else {
      next.delete("sector");
    }
    next.delete("stock");
    setSearchParams(next);
    load(s);
  };

  const handleSelectStock = (sym: string) => {
    const next = new URLSearchParams(searchParams);
    if (sym && sym !== selectedStock) {
      next.set("stock", sym);
    } else {
      next.delete("stock");
    }
    setSearchParams(next);
  };

  const handleCloseDrawer = () => {
    const next = new URLSearchParams(searchParams);
    next.delete("stock");
    setSearchParams(next);
  };

  // Global keyboard shortcuts
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setCmdOpen((o) => !o);
      }
      if (e.key === "Escape" && selectedStock) {
        handleCloseDrawer();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  return (
    <div className="flex h-screen overflow-hidden bg-surface-0">
      <SidebarNav
        sectors={data?.available_sectors ?? []}
        activeSector={sector}
        onSectorChange={handleSectorChange}
        onOpenCommandPalette={() => setCmdOpen(true)}
      />

      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar */}
        <header className="h-12 flex items-center justify-between px-5 border-b border-line flex-shrink-0">
          <div className="flex items-center gap-3">
            <h1 className="text-[13px] font-medium text-content">
              {location.pathname === "/scan"
                ? "Anomaly Scan"
                : location.pathname === "/settings"
                  ? "Settings"
                  : "S&P 500 Dashboard"}
            </h1>
            {data && location.pathname === "/" && (
              <span className="text-2xs text-content-muted tabular-nums">
                {data.total_companies} companies
                {data.as_of && ` · ${data.as_of}`}
              </span>
            )}
          </div>
          <button
            onClick={() => setCmdOpen(true)}
            className="flex items-center gap-1.5 text-xs text-content-muted hover:text-content-secondary transition-colors"
          >
            Search
            <kbd className="text-2xs bg-surface-2 px-1.5 py-0.5 rounded border border-line">
              ⌘K
            </kbd>
          </button>
        </header>

        {/* Content + drawer */}
        <div className="flex-1 flex min-h-0">
          <main className="flex-1 overflow-auto min-w-0">
            <Outlet
              context={{
                data,
                loading,
                error,
                selectedStock,
                onSelectStock: handleSelectStock,
                onRefresh: () => load(),
              }}
            />
          </main>

          {selectedCompany && (
            <DetailDrawer
              company={selectedCompany}
              onClose={handleCloseDrawer}
            />
          )}
        </div>
      </div>

      <CommandPalette
        open={cmdOpen}
        onClose={() => setCmdOpen(false)}
        companies={allCompanies}
        onSelectStock={handleSelectStock}
        onRefresh={() => load()}
      />

      <ToastContainer />
    </div>
  );
}

export interface DashboardOutletContext {
  data: DashboardData | null;
  loading: boolean;
  error: string | null;
  selectedStock: string;
  onSelectStock: (sym: string) => void;
  onRefresh: () => void;
}
