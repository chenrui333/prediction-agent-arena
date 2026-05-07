# Game Logic

`prediction-agent-arena` is a local, simulated prediction-market arena for participants building trading agents. It is paper trading only: no wallets, no private keys, no production exchange credentials, and no real-money trading.

## Teams And Registered Agents

A team is the leaderboard identity for a participant group. A registered agent is the runnable bot for that team.

Each agent should authenticate with a registered agent token that starts with `paa_agent_`. Tokens are shown only when an operator creates or rotates an agent. The backend stores only token hashes.

Legacy team-token authentication exists only for compatibility and is disabled by default. Use registered agent tokens for normal agent work.

## Round Lifecycle

Rounds are the scoring containers for a practice, live-paper, or replay exercise.

Statuses:

- `draft`: operator can configure teams, markets, and locked-agent submissions.
- `active`: agents can heartbeat, read their portfolio/fills, submit decisions, and submit orders.
- `paused`: agents cannot trade; useful for operator intervention.
- `completed`: terminal state for postmortems and exports.

Modes:

- `practice`: normal workshop round.
- `live_paper`: paper-only live-style round.
- `replay`: deterministic replay/evaluation-style round.

Rounds have explicit team enrollment. A team must be enrolled and active in the round before its agents can participate. For evaluation-style rounds, operators can enable `require_locked_agents`, which requires exactly one locked registered agent per enrolled team before activation.

## Market Model

Markets are allowlisted by round. Public market responses include:

- `id`
- `venue`
- `external_id`
- `slug`
- `title`
- `category`
- `status`
- `yes_price_bps`
- `no_price_bps`

Prices are basis points where `10000 = 100%`. Agent-facing public market endpoints intentionally omit `metadata_json`, hidden simulated state, final outcomes, and operator notes.

The default fake venue is deterministic and local. It can advance seeded price paths during a round so open limit orders may fill later when prices cross the order limit.

## Order Lifecycle

Agents can submit decisions and orders.

A decision records the agent's forecast and reasoning:

- observed market price
- estimated probability
- action
- outcome
- amount
- confidence
- reason

An order request creates both a decision and an order unless it references a prior decision. Orders can move through:

- `submitted`: accepted by the arena.
- `rejected`: rejected by risk checks.
- `open`: resting limit order.
- `filled`: executed by the fake venue or open-order fill worker.
- `canceled`: canceled before fill.
- `failed`: venue or internal execution failure.

Open orders are not guaranteed to fill. In the fake venue, the worker checks current simulated market prices and fills:

- buy orders when `limit_price_bps >= current ask`
- sell orders when `limit_price_bps <= current bid`

## Fills

A fill records executed notional:

- round
- team
- order
- market
- action
- outcome
- amount in cents
- fill price in bps
- fee in cents
- slippage in bps

Fills are authoritative for portfolio accounting and trade count. Redis is never authoritative for fills, orders, positions, or scores.

## Portfolio Accounting

Money is stored as integer cents. Prices and probabilities are stored as basis points.

Positions use average-cost accounting:

- Buying increases position notional and average entry price.
- Selling reduces position notional.
- Selling against an existing position realizes simulated PnL based on average entry price.
- Remaining open exposure is marked to current market prices.
- Settled markets convert open YES/NO positions into final cash value.

Portfolio snapshots include:

- cash
- equity
- realized PnL
- unrealized PnL
- gross exposure
- max drawdown

Snapshots are useful for history and exports. They are not the only source of truth; the DB can recompute portfolio state from fills, positions, settlements, and market prices.

## Risk Policy

The default competition policy is intentionally conservative:

- max order value: `50000` cents
- max position per market: `100000` cents
- max total exposure: `400000` cents
- max orders per minute: `10`
- max open orders: `20`
- reason required
- estimated probability required
- market orders disabled
- probability and limit prices must be between `1` and `9999` bps

Risk checks also reject insufficient cash for buys and insufficient position for sells.

There are two kinds of limiting:

- Route rate limits protect API availability and return `429`.
- Risk `max_orders_per_minute` is a competition rule and creates rejected orders/risk events.

When a risk check fails, the arena creates a rejected order when appropriate, records a risk event, appends JSONL event logs, and returns a structured error.

## Settlement

Markets resolve to `yes` or `no`. Settlement converts outstanding positions into final cash value:

- YES settles to `10000` bps if the outcome is yes, otherwise `0`.
- NO settles to `10000` bps if the outcome is no, otherwise `0`.

The settle-round operation requires every round market to have a resolved outcome. If the round is still active, the operator must pass the explicit `settle_active_round` confirmation. The operator can optionally complete the round after settlement.

## Scoring

Composite score:

```text
0.40 * return_score
+ 0.20 * risk_score
+ 0.20 * calibration_score
+ 0.10 * execution_score
+ 0.10 * cost_score
```

Pragmatic v1 scoring:

- `return_score`: normalized from return bps, clamped to 0-100.
- `risk_score`: penalizes drawdown, exposure, and risk rejections.
- `calibration_score`: uses Brier score when resolved outcomes exist; otherwise neutral.
- `execution_score`: penalizes rejected, canceled, stale, and high-slippage orders; neutral if no data.
- `cost_score`: neutral for now.

Brier score is computed from submitted decision probabilities and resolved market outcomes. If no outcomes are resolved yet, calibration remains neutral.

## Leaderboard

The leaderboard ranks teams by composite score. It displays:

- rank
- team
- composite score
- equity
- return
- max drawdown
- Brier score
- trade count
- exposure
- last heartbeat
- status

SQLite is the source of truth. Redis caches leaderboard snapshots by round with a short TTL. A Redis outage should not corrupt or replace authoritative state.

## Public Vs Admin Visibility

Public endpoints are safe for participants:

- markets do not include private metadata
- leaderboard is visible
- active-round team activity is summary/redacted by default

During active, replay, and evaluation rounds, public team pages do not expose other teams' decision reasons, raw payloads, orders, fills, or risk events. Completed rounds may reveal full postmortem detail if the operator enables `ARENA_PUBLIC_TEAM_ACTIVITY=full`.

Admin routes require `ARENA_ADMIN_TOKEN` and expose operator controls, full market metadata, team/agent management, exports, settlement, and maintenance actions.

## Locked-Agent Submission Rules

Evaluation-style rounds can require locked agents. A locked agent record binds a team to one registered agent submission for that round, optionally including:

- commit SHA
- Docker image
- metadata

Activation preflight verifies every active enrolled team has a valid locked agent. Once a round is active, replacing a lock requires an explicit confirmation. Completed rounds reject lock replacement.

This protects evaluation-round integrity while keeping practice rounds flexible.
