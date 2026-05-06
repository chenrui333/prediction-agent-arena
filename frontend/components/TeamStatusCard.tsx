import Link from "next/link";
import type { LeaderboardRow } from "@/lib/types";
import { formatMoney, formatBps } from "@/lib/api";

export function TeamStatusCard({ row }: { row: LeaderboardRow }) {
  return (
    <section className="panel stack">
      <h2>{row.team_name}</h2>
      <div className="grid">
        <div>
          <h3>Rank</h3>
          <p className="rank">#{row.rank}</p>
        </div>
        <div>
          <h3>Score</h3>
          <p className="score">{row.composite_score}</p>
        </div>
        <div>
          <h3>Equity</h3>
          <p>{formatMoney(row.equity_cents)}</p>
        </div>
        <div>
          <h3>Return</h3>
          <p className={row.return_bps >= 0 ? "positive" : "negative"}>{formatBps(row.return_bps)}</p>
        </div>
      </div>
      <div className="actions">
        <Link className="button" href="/leaderboard">
          Back to leaderboard
        </Link>
      </div>
    </section>
  );
}
