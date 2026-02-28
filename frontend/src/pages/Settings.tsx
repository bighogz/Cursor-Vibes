import { useEffect, useState } from "react";
import { fetchHealth, fetchProviders, refreshDashboard } from "../lib/api";
import { useToast } from "../context/toast-context";
import { IconRefresh, IconSettings } from "../components/icons";

export function Settings() {
  const { toast } = useToast();
  const [health, setHealth] = useState<Record<string, unknown> | null>(null);
  const [providers, setProviders] = useState<Record<string, unknown> | null>(
    null
  );

  useEffect(() => {
    fetchHealth().then(setHealth).catch(console.error);
    fetchProviders().then(setProviders).catch(console.error);
  }, []);

  async function handleRefresh() {
    try {
      await refreshDashboard();
      toast("Dashboard refresh triggered", "success");
    } catch {
      toast("Failed to trigger refresh", "error");
    }
  }

  return (
    <div className="max-w-2xl mx-auto px-5 py-6">
      <div className="flex items-center gap-3 mb-6">
        <IconSettings size={20} className="text-accent" />
        <h2 className="text-base font-semibold text-content">Settings</h2>
      </div>

      {/* Server health */}
      <Section title="Server Health">
        <div className="bg-surface-1 border border-line rounded-lg p-4">
          {health ? (
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-positive" />
              <span className="text-[13px] text-content-secondary">
                Server is{" "}
                <span className="text-content font-medium">
                  {(health.status as string) ?? "unknown"}
                </span>
              </span>
            </div>
          ) : (
            <span className="text-[13px] text-content-muted">Loading…</span>
          )}
        </div>
      </Section>

      {/* Provider diagnostics */}
      <Section title="Provider Diagnostics">
        <div className="bg-surface-1 border border-line rounded-lg p-4">
          {providers ? (
            <pre className="text-xs text-content-secondary whitespace-pre-wrap font-mono leading-relaxed overflow-x-auto">
              {JSON.stringify(providers, null, 2)}
            </pre>
          ) : (
            <span className="text-[13px] text-content-muted">Loading…</span>
          )}
        </div>
      </Section>

      {/* Actions */}
      <Section title="Actions">
        <button
          onClick={handleRefresh}
          className="flex items-center gap-2 px-4 py-2 bg-accent hover:bg-accent-hover text-white
                     text-[13px] font-medium rounded-md transition-colors"
        >
          <IconRefresh size={14} />
          Force Dashboard Refresh
        </button>
        <p className="mt-2 text-2xs text-content-muted">
          Triggers a full cache rebuild. Data will be fresh within 2-3 minutes.
        </p>
      </Section>

      {/* About */}
      <Section title="About">
        <div className="text-[13px] text-content-secondary space-y-1">
          <p>
            <strong className="text-content">Vibes</strong> — S&P 500 Insider
            Selling Tracker
          </p>
          <p>Go + Rust backend, React frontend</p>
          <p className="text-content-muted text-xs">v1.0.0</p>
        </div>
      </Section>
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
    <div className="mb-6">
      <h3 className="text-2xs font-medium uppercase tracking-wider text-content-muted mb-3">
        {title}
      </h3>
      {children}
    </div>
  );
}
