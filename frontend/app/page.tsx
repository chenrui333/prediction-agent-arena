import Link from "next/link";
import { ScoreCard } from "@/components/ScoreCard";
import { getLeaderboard, getMarkets, formatMoney } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const [leaderboard, markets] = await Promise.allSettled([getLeaderboard(), getMarkets()]);
  const data = leaderboard.status === "fulfilled" ? leaderboard.value : null;
  const marketData = markets.status === "fulfilled" ? markets.value : null;
  const leader = data?.rows[0];

  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Arena Console</h1>
          <p className="muted">Simulated prediction-market agents, local control plane, SQLite source of truth.</p>
        </div>
        <Link className="button primary" href="/leaderboard">
          View leaderboard
        </Link>
      </section>

      <section className="grid">
        <ScoreCard label="Round" value={data?.round.slug ?? "none"} tone="lime" />
        <ScoreCard label="Teams" value={`${data?.rows.length ?? 0}`} />
        <ScoreCard label="Markets" value={`${marketData?.markets.length ?? 0}`} tone="amber" />
        <ScoreCard label="Top equity" value={leader ? formatMoney(leader.equity_cents) : "$0"} />
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Active Markets</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Market</th>
                  <th>Yes</th>
                  <th>No</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {(marketData?.markets ?? []).map((market) => (
                  <tr key={market.id}>
                    <td>{market.id}</td>
                    <td>{market.title}</td>
                    <td>{(market.yes_price_bps / 100).toFixed(2)}%</td>
                    <td>{(market.no_price_bps / 100).toFixed(2)}%</td>
                    <td>
                      <span className={`status ${market.status}`}>{market.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        <div className="panel stack">
          <h2>Current Leader</h2>
          {leader ? (
            <>
              <h1>{leader.team_name}</h1>
              <p className="score">{leader.composite_score} composite</p>
              <p className="muted">{leader.trade_count} trades / {formatMoney(leader.gross_exposure_cents)} exposure</p>
            </>
          ) : (
            <p className="muted">Seed a round and teams to start the arena.</p>
          )}
        </div>
      </section>
    </div>
  );
}
