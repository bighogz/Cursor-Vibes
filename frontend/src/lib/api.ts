import type { DashboardData, ScanResult } from "../types/dashboard";

const BASE = "";

export async function fetchDashboard(
  sector?: string,
  limit?: number
): Promise<DashboardData> {
  const params = new URLSearchParams();
  if (sector) params.set("sector", sector);
  params.set("limit", String(limit ?? 50));
  const qs = params.toString();
  const res = await fetch(`${BASE}/api/dashboard${qs ? "?" + qs : ""}`);
  if (!res.ok) throw new Error(`Dashboard fetch failed: ${res.status}`);
  return res.json();
}

export async function refreshDashboard(): Promise<void> {
  await fetch(`${BASE}/api/dashboard/refresh`, { method: "POST" });
}

export async function fetchScan(opts?: {
  limit?: number;
  baseline_days?: number;
  current_days?: number;
  std_threshold?: number;
}): Promise<ScanResult> {
  const params = new URLSearchParams();
  if (opts?.limit) params.set("limit", String(opts.limit));
  if (opts?.baseline_days)
    params.set("baseline_days", String(opts.baseline_days));
  if (opts?.current_days)
    params.set("current_days", String(opts.current_days));
  if (opts?.std_threshold)
    params.set("std_threshold", String(opts.std_threshold));
  const qs = params.toString();
  const res = await fetch(`${BASE}/api/scan${qs ? "?" + qs : ""}`, {
    method: "POST",
  });
  if (!res.ok) throw new Error(`Scan failed: ${res.status}`);
  return res.json();
}

export async function fetchHealth(): Promise<{ status: string }> {
  const res = await fetch(`${BASE}/api/health`);
  return res.json();
}

export async function fetchProviders(): Promise<Record<string, unknown>> {
  const res = await fetch(`${BASE}/api/health/providers`);
  return res.json();
}
