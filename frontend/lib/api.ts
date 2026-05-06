import type { AdminSummary, Agent, ArenaHealth, LeaderboardResponse, MarketsResponse, Team, Round, Market, TeamActivity } from "./types";

export const apiBase = process.env.NEXT_PUBLIC_ARENA_API_BASE_URL ?? "http://localhost:8080";

export class ArenaAPIError extends Error {
  code: string;
  details?: Record<string, unknown>;
  status: number;

  constructor(status: number, code: string, message: string, details?: Record<string, unknown>) {
    super(message);
    this.name = "ArenaAPIError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  if (!response.ok) {
    const text = await response.text();
    throw parseAPIError(response.status, text);
  }
  return response.json() as Promise<T>;
}

export function getLeaderboard(): Promise<LeaderboardResponse> {
  return fetchJSON<LeaderboardResponse>("/api/v1/leaderboard");
}

export function getMarkets(): Promise<MarketsResponse> {
  return fetchJSON<MarketsResponse>("/api/v1/markets");
}

export function getTeams(adminToken: string): Promise<Team[]> {
  return fetchJSON<Team[]>("/api/v1/admin/teams", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getRounds(adminToken: string): Promise<Round[]> {
  return fetchJSON<Round[]>("/api/v1/admin/rounds", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getAdminMarkets(adminToken: string): Promise<Market[]> {
  return fetchJSON<Market[]>("/api/v1/admin/markets", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getAdminSummary(adminToken: string): Promise<AdminSummary> {
  return fetchJSON<AdminSummary>("/api/v1/admin/summary", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getAdminHealth(adminToken: string): Promise<ArenaHealth> {
  return fetchJSON<ArenaHealth>("/api/v1/admin/health", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getTeamAgents(adminToken: string, teamID: number): Promise<Agent[]> {
  return fetchJSON<Agent[]>(`/api/v1/admin/teams/${teamID}/agents`, {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
}

export function getTeamActivity(teamSlug: string): Promise<TeamActivity> {
  return fetchJSON<TeamActivity>(`/api/v1/teams/${teamSlug}`);
}

export function formatAPIError(error: unknown): string {
  if (error instanceof ArenaAPIError) {
    return error.code ? `${error.code}: ${error.message}` : error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "request failed";
}

export function formatDateTime(value?: string): string {
  if (!value) {
    return "-";
  }
  return new Date(value).toLocaleString();
}

export function formatTime(value?: string): string {
  if (!value) {
    return "-";
  }
  return new Date(value).toLocaleTimeString();
}

export function formatMoney(cents: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(cents / 100);
}

export function formatBps(bps: number): string {
  const sign = bps > 0 ? "+" : "";
  return `${sign}${(bps / 100).toFixed(2)}%`;
}

export function formatPercentBps(bps: number): string {
  return `${(bps / 100).toFixed(2)}%`;
}

function parseAPIError(status: number, text: string): ArenaAPIError {
  if (!text) {
    return new ArenaAPIError(status, `http_${status}`, `request failed with ${status}`);
  }
  try {
    const parsed = JSON.parse(text) as {
      error?: {
        code?: string;
        message?: string;
        details?: Record<string, unknown>;
      };
    };
    if (parsed.error?.message) {
      return new ArenaAPIError(status, parsed.error.code ?? `http_${status}`, parsed.error.message, parsed.error.details);
    }
  } catch {
    // Fall through to plain-text error.
  }
  return new ArenaAPIError(status, `http_${status}`, text);
}
