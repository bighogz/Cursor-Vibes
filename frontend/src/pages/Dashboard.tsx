import { useOutletContext } from "react-router-dom";
import type { DashboardOutletContext } from "../components/AppShell";
import { DataTable } from "../components/DataTable";
import { IconRefresh } from "../components/icons";

export function Dashboard() {
  const { data, loading, error, selectedStock, onSelectStock, onRefresh } =
    useOutletContext<DashboardOutletContext>();

  return (
    <div className="h-full flex flex-col">
      {/* Error banner */}
      {error && (
        <div className="mx-5 mt-4 px-4 py-3 rounded-lg bg-negative-dim border border-negative/20 text-negative text-[13px] flex items-center justify-between">
          <span>{error}</span>
          <button
            onClick={onRefresh}
            className="flex items-center gap-1.5 text-xs font-medium hover:underline"
          >
            <IconRefresh size={14} />
            Retry
          </button>
        </div>
      )}

      {/* Provider status */}
      {data?.provider_status &&
        Object.keys(data.provider_status).length > 0 && (
          <div className="mx-5 mt-3 px-4 py-2 rounded-lg bg-surface-1 border border-line text-2xs text-content-muted flex items-center gap-3">
            <span className="font-medium">Provider status:</span>
            {Object.entries(data.provider_status).map(([k, v]) => (
              <span key={k}>
                {k}={v}
              </span>
            ))}
          </div>
        )}

      {/* Table */}
      <div className="flex-1 overflow-auto mt-1">
        <DataTable
          sectors={data?.sectors ?? []}
          selectedStock={selectedStock}
          onSelectStock={onSelectStock}
          loading={loading}
        />
      </div>
    </div>
  );
}
