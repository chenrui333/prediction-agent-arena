import Link from "next/link";
import { ScoreCard } from "@/components/ScoreCard";
import { formatAPIError, formatMoney, getLeaderboard, getMarkets } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const [leaderboard, markets] = await Promise.allSettled([getLeaderboard(), getMarkets()]);
  const data = leaderboard.status === "fulfilled" ? leaderboard.value : null;
  const marketData = markets.status === "fulfilled" ? markets.value : null;
  const leader = data?.rows[0];
  const backendError = leaderboard.status === "rejected" ? formatAPIError(leaderboard.reason) : markets.status === "rejected" ? formatAPIError(markets.reason) : "";

  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>Arena Console</h1>
          <p className="muted">Two-week bootcamp control plane for simulated prediction-market agents.</p>
        </div>
        <div className="actions">
          <Link className="button primary" href="/leaderboard">
            View leaderboard
          </Link>
          <Link className="button" href="/admin">
            Admin console
          </Link>
        </div>
      </section>

      {backendError ? <div className="notice error">Backend API unavailable: {backendError}</div> : null}

      <section className="grid">
        <ScoreCard label="Active Round" value={data?.round.slug ?? "none"} tone="lime" />
        <ScoreCard label="Teams" value={`${data?.rows.length ?? 0}`} />
        <ScoreCard label="Markets" value={`${marketData?.markets.length ?? 0}`} tone="amber" />
        <ScoreCard label="Top equity" value={leader ? formatMoney(leader.equity_cents) : "$0"} />
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Course Arena</h2>
          <p className="muted">
            Students run local agents against an instructor-hosted Go API. The arena records decisions, paper orders, fills, risk events, portfolios,
            and scores without wallets or real-money exchange access.
          </p>
          <div className="actions">
            <Link className="button" href="/leaderboard">
              Project leaderboard
            </Link>
            <Link className="button" href="/admin">
              Manage rounds
            </Link>
          </div>
        </div>
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
                {(marketData?.markets ?? []).length === 0 ? (
                  <tr>
                    <td colSpan={5} className="muted">
                      No markets are available. Seed demo state or activate a round.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section className="split">
        <div className="panel stack">
          <h2>Active Round Summary</h2>
          {data ? (
            <>
              <h3>{data.round.status}</h3>
              <p className="score">{data.round.name}</p>
              <p className="muted">
                {data.rows.length} teams / {marketData?.markets.length ?? 0} markets / initial balance {formatMoney(data.round.initial_balance_cents)}
              </p>
            </>
          ) : (
            <p className="muted">No active round is available yet.</p>
          )}
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
