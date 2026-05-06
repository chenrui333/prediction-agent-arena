"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { formatBps, formatMoney } from "@/lib/api";
import type { LeaderboardResponse } from "@/lib/types";

type Props = {
  initial: LeaderboardResponse;
  apiBase: string;
  refreshMs?: number;
};

export function FinalsLeaderboard({ initial, apiBase, refreshMs = 5000 }: Props) {
  const [data, setData] = useState(initial);
  const [error, setError] = useState("");
  const [updatedAt, setUpdatedAt] = useState(new Date());
  const topThree = useMemo(() => data.rows.slice(0, 3), [data.rows]);
  const isFinal = data.round.status === "completed";
  const isPaused = data.round.status === "paused";

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const response = await fetch(`${apiBase}/api/v1/leaderboard`, { cache: "no-store" });
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }
        const next = (await response.json()) as LeaderboardResponse;
        if (active) {
          setData(next);
          setUpdatedAt(new Date());
          setError("");
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : "refresh failed");
        }
      }
    };
    const id = window.setInterval(load, refreshMs);
    return () => {
      active = false;
      window.clearInterval(id);
    };
  }, [apiBase, refreshMs]);

  return (
    <div className="stack finals-view">
      <section className="page-head">
        <div>
          <h1>Finals</h1>
          <p className="muted">
            {data.round.name} / {data.round.status} / updated {updatedAt.toLocaleTimeString()}
          </p>
        </div>
        <div className="actions">
          <Link className="button" href="/leaderboard">
            Full leaderboard
          </Link>
        </div>
      </section>

      <div className={`notice ${isFinal ? "" : isPaused ? "error" : ""}`}>
        {isFinal ? "Final leaderboard is completed or frozen for post-round review." : isPaused ? "Round is paused." : "Round is live."}
      </div>
      {error ? <div className="notice error">Refresh failed: {error}</div> : null}

      {topThree.length > 0 ? (
        <section className="podium-grid">
          {topThree.map((row) => (
            <div key={row.team_id} className="podium-card">
              <span className="rank">#{row.rank}</span>
              <strong>{row.team_name}</strong>
              <span className="score">{row.composite_score}</span>
              <span className="muted">
                {formatMoney(row.equity_cents)} / {formatBps(row.return_bps)}
              </span>
            </div>
          ))}
        </section>
      ) : null}

      <section className="table-wrap finals-table">
        <table>
          <thead>
            <tr>
              <th>Rank</th>
              <th>Team</th>
              <th>Score</th>
              <th>Equity</th>
              <th>Return</th>
              <th>Trades</th>
              <th>Heartbeat</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {data.rows.map((row) => (
              <tr key={row.team_id}>
                <td className="rank">#{row.rank}</td>
                <td>
                  <Link href={`/teams/${row.team_slug}`}>{row.team_name}</Link>
                </td>
                <td className="score">{row.composite_score}</td>
                <td>{formatMoney(row.equity_cents)}</td>
                <td className={row.return_bps >= 0 ? "positive" : "negative"}>{formatBps(row.return_bps)}</td>
                <td>{row.trade_count}</td>
                <td>{row.last_heartbeat ? new Date(row.last_heartbeat).toLocaleTimeString() : "-"}</td>
                <td>
                  <span className={`status ${row.status}`}>{row.status}</span>
                </td>
              </tr>
            ))}
            {data.rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="muted">
                  No finalists are ranked yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </section>
    </div>
  );
}
