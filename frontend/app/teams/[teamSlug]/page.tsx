import { notFound } from "next/navigation";
import { TeamStatusCard } from "@/components/TeamStatusCard";
import { getLeaderboard, getTeamActivity, formatBps, formatMoney } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function TeamPage({ params }: { params: Promise<{ teamSlug: string }> }) {
  const { teamSlug } = await params;
  const [leaderboard, activity] = await Promise.all([getLeaderboard(), getTeamActivity(teamSlug)]);
  const row = leaderboard.rows.find((item) => item.team_slug === teamSlug);
  if (!row) {
    notFound();
  }
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>{row.team_name}</h1>
          <p className="muted">
            {leaderboard.round.name} / rank #{row.rank} / status {row.status}
          </p>
        </div>
      </section>
      <TeamStatusCard row={row} />
      <section className="grid">
        <div className="metric">
          <span className="label">Exposure</span>
          <span className="value">{formatMoney(activity.portfolio.gross_exposure_cents)}</span>
        </div>
        <div className="metric">
          <span className="label">Drawdown</span>
          <span className="value negative">{formatBps(-activity.portfolio.max_drawdown_bps)}</span>
        </div>
        <div className="metric">
          <span className="label">Cash</span>
          <span className="value">{formatMoney(activity.portfolio.cash_cents)}</span>
        </div>
        <div className="metric">
          <span className="label">Heartbeat</span>
          <span className="value">{row.last_heartbeat ? new Date(row.last_heartbeat).toLocaleTimeString() : "-"}</span>
        </div>
      </section>
      <section className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Recent Decisions</th>
              <th>Market</th>
              <th>Action</th>
              <th>Estimate</th>
              <th>Edge</th>
              <th>Amount</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {activity.decisions.map((decision) => (
              <tr key={decision.id}>
                <td>{new Date(decision.created_at).toLocaleTimeString()}</td>
                <td>#{decision.market_id}</td>
                <td>
                  {decision.action} {decision.outcome}
                </td>
                <td>{decision.estimated_probability_bps ? formatBps(decision.estimated_probability_bps) : "-"}</td>
                <td>{formatBps(decision.edge_bps)}</td>
                <td>{formatMoney(decision.amount_cents)}</td>
                <td className="wrap">{decision.reason}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Recent Orders</th>
              <th>Market</th>
              <th>Side</th>
              <th>Limit</th>
              <th>Amount</th>
              <th>Status</th>
              <th>Rejection</th>
            </tr>
          </thead>
          <tbody>
            {activity.orders.map((order) => (
              <tr key={order.id}>
                <td>{new Date(order.created_at).toLocaleTimeString()}</td>
                <td>#{order.market_id}</td>
                <td>
                  {order.action} {order.outcome}
                </td>
                <td>{formatBps(order.limit_price_bps)}</td>
                <td>{formatMoney(order.amount_cents)}</td>
                <td>
                  <span className={`status ${order.status}`}>{order.status}</span>
                </td>
                <td className="wrap">{order.rejection_reason || "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="split">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Fills</th>
                <th>Market</th>
                <th>Side</th>
                <th>Price</th>
                <th>Amount</th>
                <th>Slippage</th>
              </tr>
            </thead>
            <tbody>
              {activity.fills.map((fill) => (
                <tr key={fill.id}>
                  <td>{new Date(fill.created_at).toLocaleTimeString()}</td>
                  <td>#{fill.market_id}</td>
                  <td>
                    {fill.action} {fill.outcome}
                  </td>
                  <td>{formatBps(fill.fill_price_bps)}</td>
                  <td>{formatMoney(fill.amount_cents)}</td>
                  <td>{formatBps(fill.slippage_bps)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Risk Events</th>
                <th>Type</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {activity.risk_events.map((event) => (
                <tr key={event.id}>
                  <td>{new Date(event.created_at).toLocaleTimeString()}</td>
                  <td>{event.type}</td>
                  <td className="wrap">{event.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
