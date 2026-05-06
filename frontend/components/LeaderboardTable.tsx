"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import type { LeaderboardResponse } from "@/lib/types";
import { formatBps, formatMoney } from "@/lib/api";

type Props = {
  initial: LeaderboardResponse;
  apiBase: string;
  refreshMs?: number;
};

export function LeaderboardTable({ initial, apiBase, refreshMs = 7000 }: Props) {
  const [data, setData] = useState(initial);
  const [error, setError] = useState("");
  const [updatedAt, setUpdatedAt] = useState(new Date());

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

  const top = useMemo(() => data.rows[0], [data.rows]);

  return (
    <div className="stack">
      <div className="page-head">
        <div>
          <h1>Leaderboard</h1>
          <p className="muted">
            {data.round.name} / {data.round.status} / refreshed {updatedAt.toLocaleTimeString()}
          </p>
        </div>
        {top ? <span className="score">Leader: {top.team_name}</span> : null}
      </div>
      {error ? <div className="notice error">Refresh failed: {error}</div> : null}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Rank</th>
              <th>Team</th>
              <th>Composite</th>
              <th>Equity</th>
              <th>Return</th>
              <th>Drawdown</th>
              <th>Brier</th>
              <th>Trades</th>
              <th>Exposure</th>
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
                <td>{formatBps(-row.max_drawdown_bps)}</td>
                <td>{(row.brier_score_bps / 100).toFixed(2)}</td>
                <td>{row.trade_count}</td>
                <td>{formatMoney(row.gross_exposure_cents)}</td>
                <td>{row.last_heartbeat ? new Date(row.last_heartbeat).toLocaleTimeString() : "-"}</td>
                <td>
                  <span className={`status ${row.status}`}>{row.status}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
