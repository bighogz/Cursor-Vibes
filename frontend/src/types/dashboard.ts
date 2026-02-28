export interface Company {
  symbol: string;
  name: string;
  price: number | null;
  change_pct: number | null;
  quarter_trend: number | null;
  quarter_closes: number[] | null;
  news: NewsItem[] | null;
  top_insiders: Insider[] | null;
  sources: Record<string, string>;
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

export interface ScanSignal {
  ticker: string;
  current_shares_sold: number;
  baseline_mean: number;
  baseline_std: number;
  z_score: number;
  is_anomaly: boolean;
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
