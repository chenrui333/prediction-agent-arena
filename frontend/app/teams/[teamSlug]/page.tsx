import { notFound } from "next/navigation";
import { TeamStatusCard } from "@/components/TeamStatusCard";
import { ArenaAPIError, formatBps, formatDateTime, formatMoney, formatPercentBps, formatTime, getLeaderboard, getTeamActivity } from "@/lib/api";
import type { LeaderboardResponse, TeamActivity } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function TeamPage({ params }: { params: Promise<{ teamSlug: string }> }) {
  const { teamSlug } = await params;
  let leaderboard: LeaderboardResponse;
  let activity: TeamActivity;
  try {
    [leaderboard, activity] = await Promise.all([getLeaderboard(), getTeamActivity(teamSlug)]);
  } catch (err) {
    if (err instanceof ArenaAPIError && err.status === 404) {
      notFound();
    }
    throw err;
  }
  const row = leaderboard.rows.find((item) => item.team_slug === teamSlug);
  if (!row) {
    notFound();
  }
  const detailRedacted = activity.detail_redacted ?? false;
  return (
    <div className="stack">
      <section className="page-head">
        <div>
          <h1>{row.team_name}</h1>
          <p className="muted">
            {leaderboard.round.name} / rank #{row.rank} / status {row.status} / heartbeat {formatTime(row.last_heartbeat)}
          </p>
        </div>
      </section>
      <TeamStatusCard row={row} />
      {detailRedacted ? (
        <section className="notice">
          Detailed decisions, orders, fills, and risk events are hidden for this round view. Public team pages show summary data during active
          competition rounds.
        </section>
      ) : null}
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
          <span className="label">Equity Updated</span>
          <span className="value compact">{formatTime(activity.portfolio.created_at)}</span>
        </div>
      </section>
      <section className="grid">
        <div className="metric">
          <span className="label">Equity</span>
          <span className="value">{formatMoney(activity.portfolio.equity_cents)}</span>
        </div>
        <div className="metric">
          <span className="label">Realized PnL</span>
          <span className="value">{formatMoney(activity.portfolio.realized_pnl_cents)}</span>
        </div>
        <div className="metric">
          <span className="label">Unrealized PnL</span>
          <span className="value">{formatMoney(activity.portfolio.unrealized_pnl_cents)}</span>
        </div>
        <div className="metric">
          <span className="label">Trades</span>
          <span className="value">{activity.trade_count ?? row.trade_count}</span>
        </div>
      </section>
      <section className="grid">
        <div className="metric">
          <span className="label">Risk Rejects</span>
          <span className="value">{activity.risk_rejection_count ?? activity.risk_events.length}</span>
        </div>
        <div className="metric">
          <span className="label">Last Heartbeat</span>
          <span className="value compact">{formatDateTime(activity.last_heartbeat ?? row.last_heartbeat)}</span>
        </div>
        <div className="metric">
          <span className="label">Activity View</span>
          <span className="value compact">{activity.visibility ?? "full"}</span>
        </div>
        <div className="metric">
          <span className="label">Round Policy</span>
          <span className="value compact">{activity.round.require_locked_agents ? "locked agents" : "open agents"}</span>
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
                <td>{decision.estimated_probability_bps ? formatPercentBps(decision.estimated_probability_bps) : "-"}</td>
                <td>{formatBps(decision.edge_bps)}</td>
                <td>{formatMoney(decision.amount_cents)}</td>
                <td className="wrap">{decision.reason}</td>
              </tr>
            ))}
            {activity.decisions.length === 0 ? (
              <tr>
                <td colSpan={7} className="muted">
                  {detailRedacted ? "Decision details are hidden until the instructor enables postmortem visibility." : "No decisions recorded for this round."}
                </td>
              </tr>
            ) : null}
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
                <td>{formatPercentBps(order.limit_price_bps)}</td>
                <td>{formatMoney(order.amount_cents)}</td>
                <td>
                  <span className={`status ${order.status}`}>{order.status}</span>
                </td>
                <td className="wrap">{order.rejection_reason || "-"}</td>
              </tr>
            ))}
            {activity.orders.length === 0 ? (
              <tr>
                <td colSpan={7} className="muted">
                  {detailRedacted ? "Order details are hidden until the instructor enables postmortem visibility." : "No orders recorded for this round."}
                </td>
              </tr>
            ) : null}
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
                  <td>{formatPercentBps(fill.fill_price_bps)}</td>
                  <td>{formatMoney(fill.amount_cents)}</td>
                  <td>{formatBps(fill.slippage_bps)}</td>
                </tr>
              ))}
              {activity.fills.length === 0 ? (
                <tr>
                  <td colSpan={6} className="muted">
                    {detailRedacted ? "Fill details are hidden until the instructor enables postmortem visibility." : "No fills recorded for this round."}
                  </td>
                </tr>
              ) : null}
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
              {activity.risk_events.length === 0 ? (
                <tr>
                  <td colSpan={3} className="muted">
                    {detailRedacted ? "Risk event details are hidden until the instructor enables postmortem visibility." : "No risk events recorded for this round."}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
