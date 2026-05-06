export type Round = {
  id: number;
  slug: string;
  name: string;
  mode: string;
  status: string;
  require_locked_agents: boolean;
  initial_balance_cents: number;
  starts_at?: string;
  ends_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type Market = {
  id: number;
  venue: string;
  external_id: string;
  slug: string;
  title: string;
  category: string;
  status: string;
  yes_price_bps: number;
  no_price_bps: number;
};

export type LeaderboardRow = {
  rank: number;
  team_id: number;
  team_slug: string;
  team_name: string;
  composite_score: number;
  equity_cents: number;
  return_bps: number;
  max_drawdown_bps: number;
  brier_score_bps: number;
  trade_count: number;
  gross_exposure_cents: number;
  last_heartbeat?: string;
  status: string;
};

export type LeaderboardResponse = {
  round: Round;
  rows: LeaderboardRow[];
};

export type Team = {
  id: number;
  slug: string;
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type Agent = {
  id: number;
  team_id: number;
  team_slug?: string;
  slug: string;
  name: string;
  status: "active" | "paused" | "revoked";
  kind: string;
  repo_url?: string;
  commit_sha?: string;
  docker_image?: string;
  metadata_json?: string;
  created_at: string;
  updated_at: string;
};

export type MarketsResponse = {
  round: Round | null;
  markets: Market[];
};

export type PortfolioSnapshot = {
  cash_cents: number;
  equity_cents: number;
  realized_pnl_cents: number;
  unrealized_pnl_cents: number;
  gross_exposure_cents: number;
  max_drawdown_bps: number;
  created_at: string;
};

export type Decision = {
  id: number;
  agent_id?: number;
  market_id: number;
  observed_price_bps: number;
  estimated_probability_bps?: number;
  edge_bps: number;
  action: string;
  outcome: string;
  amount_cents: number;
  confidence: string;
  reason: string;
  created_at: string;
};

export type Order = {
  id: number;
  agent_id?: number;
  market_id: number;
  action: string;
  outcome: string;
  amount_cents: number;
  limit_price_bps: number;
  status: string;
  rejection_reason?: string;
  created_at: string;
};

export type Fill = {
  id: number;
  order_id: number;
  market_id: number;
  action: string;
  outcome: string;
  amount_cents: number;
  fill_price_bps: number;
  fee_cents: number;
  slippage_bps: number;
  created_at: string;
};

export type RiskEvent = {
  id: number;
  agent_id?: number;
  order_id?: number;
  type: string;
  message: string;
  created_at: string;
};

export type AdminTeamStats = {
  team_id: number;
  team_slug: string;
  team_name: string;
  is_active: boolean;
  status: string;
  last_heartbeat?: string;
  equity_cents: number;
  trade_count: number;
  risk_rejection_count: number;
  gross_exposure_cents: number;
};

export type AdminSummary = {
  active_round: Round | null;
  latest_round: Round | null;
  teams: AdminTeamStats[];
  risk_policy: Record<string, unknown>;
};

export type ArenaHealth = {
  status: string;
  db_ok: boolean;
  redis_ok: boolean;
  active_round_id?: number;
  active_round_slug?: string;
  latest_market_tick_at?: string;
  latest_worker_heartbeat_at?: string;
  latest_portfolio_snapshot_at?: string;
};

export type TeamActivity = {
  team: Team;
  round: Round;
  portfolio: PortfolioSnapshot;
  visibility?: "summary" | "redacted" | "full";
  detail_redacted?: boolean;
  trade_count?: number;
  risk_rejection_count?: number;
  last_heartbeat?: string;
  decisions: Decision[];
  orders: Order[];
  fills: Fill[];
  risk_events: RiskEvent[];
};
