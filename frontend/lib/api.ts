import type { AdminSummary, LeaderboardResponse, MarketsResponse, Team, Round, Market, TeamActivity } from "./types";

export const apiBase = process.env.NEXT_PUBLIC_ARENA_API_BASE_URL ?? "http://localhost:8080";

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
    throw new Error(text || `request failed with ${response.status}`);
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

export function getTeamActivity(teamSlug: string): Promise<TeamActivity> {
  return fetchJSON<TeamActivity>(`/api/v1/teams/${teamSlug}`);
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
