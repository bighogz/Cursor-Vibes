export type TrendKey = "daily" | "weekly" | "monthly" | "quarterly";

export interface TrendPeriod {
  pct: number;
  closes: number[];
}

export interface Company {
  symbol: string;
  name: string;
  price: number | null;
  change_pct: number | null;
  quarter_trend: number | null;
  quarter_closes: number[] | null;
  trends?: Partial<Record<TrendKey, TrendPeriod>> | null;
  news: NewsItem[] | null;
  top_insiders: Insider[] | null;
  sources: Record<string, string>;
  anomaly_score?: number | null;
  volume_z_score?: number | null;
  breadth_z_score?: number | null;
  acceleration_score?: number | null;
  unique_insiders?: number | null;
}

export interface NewsItem {
  title: string;
  url: string;
}

export interface Insider {
  name: string;
  role?: string;
  shares: number;
  value?: number | null;
  tx_type?: string;
  source?: string;
}

export interface Sector {
  name: string;
  companies: Company[];
}

export interface DashboardData {
  as_of: string;
  total_companies: number;
  sectors: Sector[];
  available_sectors: string[];
  provider_status?: Record<string, string>;
  error?: string;
}

export interface AnomalyExplanation {
  summary: string;
  drivers: string[];
  caveats: string[];
}

export interface ScanSignal {
  ticker: string;
  composite_score: number;
  volume_z_score: number;
  breadth_z_score: number;
  acceleration_score: number;
  is_anomaly: boolean;
  current_dollar_vol: number;
  current_shares_sold: number;
  unique_insiders: number;
  baseline_mean: number;
  baseline_std: number;
}

export interface ScanResult {
  tickers_count: number;
  records_count: number;
  anomalies_count: number;
  date_from: string;
  date_to: string;
  as_of: string;
  params: {
    baseline_days: number;
    current_days: number;
    std_threshold: number;
  };
  anomalies: ScanSignal[];
  all_signals: ScanSignal[];
  error?: string;
}
