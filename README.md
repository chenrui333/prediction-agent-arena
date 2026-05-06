# prediction-agent-arena

Local simulated prediction-market agent arena for an agentic AI bootcamp. Students run trading agents from their laptops and call an instructor-run API. The arena handles teams, rounds, market allowlists, student tokens, heartbeats, decisions, orders, risk checks, fake fills, portfolio snapshots, scoring, Redis-cached leaderboards, JSONL/CSV export, and a simple live UI.

## Safety

This project is simulated/paper-trading only.

- No real-money trading.
- No wallets.
- No production exchange credentials.
- No real Polymarket private-key trading.
- No production Kalshi integration in v1.
- The default venue is a deterministic local fake venue with seeded price paths and simulated outcomes.

## Toolchain

The local toolchain is pinned in `mise.toml`:

- Go `1.26.2`
- Node `24.15.0`
- just `1.50.0`

Install tools with:

```bash
mise install
```

Then use `just` for local workflows.

## Architecture

- `backend/`: Go API and worker using `net/http`, `chi`, `database/sql`, `modernc.org/sqlite`, `go-redis`, and `slog`.
- `frontend/`: Next.js App Router + TypeScript dashboard.
- `examples/`: Go student agents with no LLM dependency.
- `scripts/`: thin Go-backed seed and export helpers.
- `data/arena.db`: local SQLite DB mounted into containers.
- `logs/{round_slug}/{team_slug}.events.jsonl`: append-only classroom event logs.
- `exports/{round_slug}/`: leaderboard CSV and score JSONL exports.

SQLite is the source of truth. Redis is only a cache and rate-limit helper. Money is stored as integer cents; prices and probabilities are basis points where `10000 = 100%`.

## Venue Configuration

The default venue is local and deterministic:

```env
ARENA_VENUE=fake
```

The fake venue reads current market prices from SQLite. Demo seed markets include deterministic price paths, and the worker advances those prices on each tick. Admins can resolve markets through the admin API; resolved outcomes feed Brier/calibration scoring.

An optional Polymarket paper-trader adapter can be selected explicitly:

```env
ARENA_VENUE=polymarket_paper
POLYMARKET_PAPER_BIN=pm-trader
POLYMARKET_PAPER_ACCOUNT_PREFIX=arena
POLYMARKET_PAPER_TIMEOUT_SECONDS=10
POLYMARKET_PAPER_DATA_DIR=/data/pm-trader
```

That adapter remains a safe skeleton in v1. It validates the configured binary and data directory but does not add wallet, private-key, or real-money trading functionality.

## Quickstart

```bash
cp .env.example .env
mise install
just docker-up
```

In another terminal:

```bash
just seed
```

Open:

- Frontend: http://localhost:3000
- Backend health: http://localhost:8080/health

`just seed` creates 10 demo teams, one active round (`practice-1`), and fake markets with deterministic price paths. It prints newly generated team tokens once. Existing tokens are never reprinted.

## Running Example Agents

Use one token printed by `just seed`:

```bash
cd examples/random-agent
ARENA_API_TOKEN=paa_... mise exec -- go run .
```

Or:

```bash
cd examples/momentum-agent
ARENA_API_TOKEN=paa_... mise exec -- go run .
```

The leaderboard refreshes automatically every 5 seconds.

## just Recipes

```bash
just test
just lint
just fmt
just seed
just docker-up
just docker-down
just logs
just export-round practice-1
just create-team team-11 "Team 11"
just create-round practice-2 "Practice Round 2"
just activate-round practice-2
just pause-team team-03
just resume-team team-03
just reset-team team-03
just freeze-leaderboard practice-1
just print-active-round
just print-team-tokens
```

`just print-team-tokens` intentionally does not dump existing secrets. Tokens are shown only when a team is created.

## Admin UI

Open http://localhost:3000/admin and enter the admin token from `.env` (`dev-admin-token` by default). The token is stored only in local browser storage for that machine and can be cleared with the `Forget token` button. The page shows active round state, teams, rounds, last heartbeat, equity, trade count, risk rejections, and exposure. It exposes pause/resume/reset team controls, round lifecycle controls, leaderboard freeze, and export.

## Frontend Pages

- `/`: course and arena overview, active round summary, markets, and links to leaderboard/admin.
- `/leaderboard`: projector-readable leaderboard with 5-second refresh and a last-updated timestamp.
- `/teams/{teamSlug}`: team summary, portfolio values, recent decisions, orders, fills, risk events, and last heartbeat. Missing teams render a normal not-found page.
- `/admin`: minimal instructor console backed by the Go admin API.

## API Examples

Admin routes require:

```http
Authorization: Bearer dev-admin-token
```

Create a team:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/teams \
  -H "Authorization: Bearer dev-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"slug":"team-11","name":"Team 11"}'
```

Student mutation and portfolio routes require:

```http
Authorization: Bearer <team_token>
```

Submit a heartbeat:

```bash
curl -sS -X POST http://localhost:8080/api/v1/heartbeat \
  -H "Authorization: Bearer $ARENA_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"online","metadata":{"agent":"my-agent"}}'
```

Submit an order:

```bash
curl -sS -X POST http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer $ARENA_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "market_id": 1,
    "outcome": "yes",
    "action": "buy",
    "amount_cents": 10000,
    "limit_price_bps": 5700,
    "estimated_probability_bps": 6400,
    "confidence": "medium",
    "reason": "My estimate is above the market implied probability."
  }'
```

Read endpoints:

```bash
curl -sS http://localhost:8080/api/v1/markets
curl -sS http://localhost:8080/api/v1/leaderboard
curl -sS -H "Authorization: Bearer $ARENA_API_TOKEN" http://localhost:8080/api/v1/portfolio
curl -sS -H "Authorization: Bearer $ARENA_API_TOKEN" http://localhost:8080/api/v1/fills
```

Structured API errors use:

```json
{
  "error": {
    "code": "risk_limit_exceeded",
    "message": "order amount exceeds max_order_value_cents",
    "details": {}
  }
}
```

## Risk Policy

Defaults:

- `initial_balance_cents`: `1000000`
- `max_order_value_cents`: `50000`
- `max_position_per_market_cents`: `100000`
- `max_total_exposure_cents`: `400000`
- `max_orders_per_minute`: `10`
- `max_open_orders`: `20`
- `require_reason`: `true`
- `require_estimated_probability`: `true`
- `allow_market_orders`: `false`
- probability and limit price ranges: `1..9999` bps

Failed checks create a rejected order when appropriate, create a risk event, append JSONL, and return a structured `400` response.

## Scoring

Composite score:

```text
0.40 * return_score
+ 0.20 * risk_score
+ 0.20 * calibration_score
+ 0.10 * execution_score
+ 0.10 * cost_score
```

V1 behavior:

- `return_score`: normalized from return bps and clamped to `0..100`.
- `risk_score`: penalizes drawdown and exposure.
- `calibration_score`: based on Brier score when markets have resolved outcomes; neutral `50` when no resolved decisions exist.
- `execution_score`: penalizes rejected orders and slippage; neutral `50` with no orders.
- `cost_score`: neutral `100`.

## SQLite and Redis Notes

- SQLite runs in WAL mode with foreign keys and `busy_timeout`.
- The backend container owns `arena.db`; SQLite is not a separate service.
- Redis stores leaderboard snapshots and short-lived rate-limit counters only.
- If Redis is unavailable, the backend logs warnings and falls back to DB computation.
- For backups and recovery steps, see `docs/instructor-runbook.md`.

## More Docs

- `docs/student-quickstart.md`
- `docs/instructor-runbook.md`
- `BOOTCAMP.md`

## Roadmap

- Round replay and richer export bundles.
- Settlement-aware portfolio accounting.
- More instructor controls for allowlists and risk policy editing.
- Optional adapter wrapping `agent-next/polymarket-paper-trader`.
- Optional Kalshi Demo venue behind the same interface.
