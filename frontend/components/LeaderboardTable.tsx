"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import type { LeaderboardResponse } from "@/lib/types";
import { fetchJSON, formatAPIError, formatBps, formatMoney, isNoActiveRoundError } from "@/lib/api";

type Props = {
	initial: LeaderboardResponse;
	refreshMs?: number;
};

export function LeaderboardTable({ initial, refreshMs = 5000 }: Props) {
	const [data, setData] = useState(initial);
	const [error, setError] = useState("");
	const [noActiveRound, setNoActiveRound] = useState(false);
	const [updatedAt, setUpdatedAt] = useState(new Date());

  useEffect(() => {
		let active = true;
		const load = async () => {
			try {
				const next = await fetchJSON<LeaderboardResponse>("/api/v1/leaderboard");
				if (active) {
					setData(next);
					setUpdatedAt(new Date());
					setError("");
					setNoActiveRound(false);
				}
			} catch (err) {
				if (active) {
					if (isNoActiveRoundError(err)) {
						setNoActiveRound(true);
						setError("");
						return;
					}
					setError(formatAPIError(err));
				}
			}
		};
    const id = window.setInterval(load, refreshMs);
    return () => {
      active = false;
      window.clearInterval(id);
    };
	}, [refreshMs]);

	const top = useMemo(() => data.rows[0], [data.rows]);

	if (noActiveRound) {
		return (
			<div className="stack">
				<div className="page-head">
					<div>
						<h1>Leaderboard</h1>
						<p className="muted">No active round is currently running.</p>
					</div>
				</div>
				<div className="notice">Agents need an active round before teams can appear on the leaderboard.</div>
			</div>
		);
	}

  return (
    <div className="stack">
      <div className="page-head">
        <div>
          <h1>Leaderboard</h1>
          <p className="muted">
            Active round: {data.round.name} / {data.round.status} / updated {updatedAt.toLocaleTimeString()}
          </p>
        </div>
        {top ? <span className="score">Leader: {top.team_name}</span> : null}
      </div>
      {error ? <div className="notice error">Refresh failed: {error}</div> : null}
      <div className="table-wrap">
        <table className="leaderboard-table">
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
            {data.rows.length === 0 ? (
              <tr>
                <td colSpan={11} className="muted">
                  No teams are on the leaderboard yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
