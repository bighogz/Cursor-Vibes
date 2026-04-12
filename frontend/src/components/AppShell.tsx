import { useCallback, useEffect, useMemo, useState } from "react";
import { Outlet, useLocation, useSearchParams } from "react-router-dom";
import { SidebarNav } from "./SidebarNav";
import { CommandPalette } from "./CommandPalette";
import { DetailDrawer } from "./DetailDrawer";
import { ToastContainer } from "./Toast";
import { fetchDashboard } from "../lib/api";
import type { Company, DashboardData, TrendKey } from "../types/dashboard";

const REFRESH_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes

export function AppShell() {
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [fullData, setFullData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [cmdOpen, setCmdOpen] = useState(false);

  const sector = searchParams.get("sector") ?? "";
  const selectedStock = searchParams.get("stock") ?? "";
  const trendPeriod = (searchParams.get("trend") ?? "quarterly") as TrendKey;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const d = await fetchDashboard();
      if (d.error) {
        setError(d.error);
      }
      setFullData(d);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load dashboard");
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch full dataset once on mount; refresh periodically in background.
  useEffect(() => {
    if (location.pathname === "/" || location.pathname === "") {
      load();
    }
  }, [location.pathname, load]);

  useEffect(() => {
    const id = setInterval(() => {
      fetchDashboard()
        .then((d) => {
          setFullData(d);
          if (d.error) setError(d.error);
        })
        .catch(() => {});
    }, REFRESH_INTERVAL_MS);
    return () => clearInterval(id);
  }, []);

  // Client-side sector filter — instant, no network call.
  const data = useMemo((): DashboardData | null => {
    if (!fullData) return null;
    if (!sector) return fullData;

    const lowerSector = sector.toLowerCase();
    const filteredSectors = fullData.sectors.filter(
      (s) => s.name.toLowerCase() === lowerSector,
    );
    const total = filteredSectors.reduce(
      (acc, s) => acc + s.companies.length,
      0,
    );
    return {
      ...fullData,
      sectors: filteredSectors,
      total_companies: total,
    };
  }, [fullData, sector]);

  const allCompanies: Company[] =
    fullData?.sectors?.flatMap((s) => s.companies) ?? [];

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
  };

  const handleTrendChange = (t: TrendKey) => {
    const next = new URLSearchParams(searchParams);
    if (t === "quarterly") {
      next.delete("trend");
    } else {
      next.set("trend", t);
    }
    setSearchParams(next);
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
        sectors={fullData?.available_sectors ?? []}
        activeSector={sector}
        onSectorChange={handleSectorChange}
        activeTrend={trendPeriod}
        onTrendChange={handleTrendChange}
        onOpenCommandPalette={() => setCmdOpen(true)}
      />

      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar */}
        <header className="h-12 flex items-center justify-between px-5 border-b border-line flex-shrink-0">
          <div className="flex items-center gap-3">
            <h1 className="text-[13px] font-medium text-content">
              {location.pathname === "/settings"
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
                trendPeriod,
                onSelectStock: handleSelectStock,
                onRefresh: () => load(),
              }}
            />
          </main>

          {selectedCompany && (
            <DetailDrawer
              company={selectedCompany}
              trendPeriod={trendPeriod}
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
  trendPeriod: TrendKey;
  onSelectStock: (sym: string) => void;
  onRefresh: () => void;
}
