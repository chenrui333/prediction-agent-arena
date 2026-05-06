# prediction-agent-arena

Local simulated prediction-market agent arena for an agentic AI bootcamp. Students run registered trading agents from their laptops and call an instructor-run API. The arena handles teams, rounds, market allowlists, agent tokens, heartbeats, decisions, orders, risk checks, fake fills, open-order fills, settlement, portfolio snapshots, scoring, Redis-cached leaderboards, JSONL/CSV export, and a simple live UI.

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
- `exports/{round_slug}/`: leaderboard CSV, score JSONL, per-team bundles, and grading reports.

SQLite is the source of truth. Redis is only a cache and rate-limit helper. Money is stored as integer cents; prices and probabilities are basis points where `10000 = 100%`. Positions use simulated contract-cents and average-cost accounting. Student API credentials are registered agent tokens (`paa_agent_...`) stored only as hashes. Legacy team-token auth is disabled by default and can be enabled only with `ARENA_LEGACY_TEAM_TOKEN_AUTH=true`.

Public team pages default to summary mode during active rounds so teams cannot inspect each other's reasoning, orders, fills, or risk events mid-competition. Instructors can use the admin API/UI for full activity. `ARENA_PUBLIC_TEAM_ACTIVITY=full` means full public postmortems after a round is completed; active rounds remain summary/redacted.

## Venue Configuration

The default venue is local and deterministic:

```env
ARENA_VENUE=fake
```

The fake venue reads current market prices from SQLite. Demo seed markets include deterministic price paths, and the worker advances those prices on each tick. Open limit orders can fill later when a price path crosses their limit. Admins can resolve markets through the admin API; resolved outcomes feed settlement and Brier/calibration scoring.

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

`just seed` creates 10 demo teams, one active round (`practice-1`), fake markets with deterministic price paths, and one default registered agent per team. It prints newly generated agent tokens once. Existing tokens are never reprinted.

## Running Example Agents

Use one `paa_agent_...` token printed by `just seed`:

```bash
cd examples/random-agent
ARENA_API_TOKEN=paa_agent_... mise exec -- go run .
```

Or:

```bash
cd examples/momentum-agent
ARENA_API_TOKEN=paa_agent_... mise exec -- go run .
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
just create-agent team-11 default "Team 11 Default Agent"
just create-round practice-2 "Practice Round 2"
just activate-round practice-2
just require-locked-agents practice-2
just allow-unlocked-agents practice-2
just pause-team team-03
just resume-team team-03
just pause-agent 1
just resume-agent 1
just revoke-agent 1
just lock-agent 1 practice-1 abc123 team-01:final
just list-round-agents practice-1
just reset-team team-03
just rotate-team-token team-03
just rotate-agent-token 1
just settle-round practice-1
just compact-snapshots practice-1
just compact-audit 14d
just backup-sqlite
just health
just freeze-leaderboard practice-1
just print-active-round
just print-team-tokens
```

`just reset-team` is round-scoped by default. Use `just reset-team-all-rounds team-03` only when you intentionally want to delete that team history across every round. `just print-team-tokens` intentionally does not dump existing secrets. Tokens are shown only when a team or agent is created or rotated. Agent tokens are the normal student credential; team tokens exist only for instructor operations and optional legacy compatibility.

## Admin UI

Open http://localhost:3000/admin and enter the admin token from `.env` (`dev-admin-token` by default for local-only mode). The token is stored only in local browser storage for that machine and can be cleared with the `Forget token` button. The page shows active round state, teams, agents, rounds, last heartbeat, equity, trade count, risk rejections, exposure, and health state. It exposes pause/resume/reset team controls, create/pause/resume/revoke/rotate agent controls, round lifecycle controls, settlement, snapshot compaction, leaderboard freeze, and export.

## Frontend Pages

- `/`: course and arena overview, active round summary, markets, and links to leaderboard/admin.
- `/leaderboard`: projector-readable leaderboard with 5-second refresh and a last-updated timestamp.
- `/teams/{teamSlug}`: public team summary, portfolio values, trade/risk counts, and last heartbeat. Detailed decisions/orders/fills/risk events are redacted during active competition rounds.
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

Create a registered agent for that team and copy the one-time `api_token`:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/teams/11/agents \
  -H "Authorization: Bearer dev-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"slug":"default","name":"Team 11 Default Agent","kind":"student"}'
```

Student mutation and portfolio routes require a registered agent token:

```http
Authorization: Bearer <agent_token>
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
curl -sS -H "Authorization: Bearer $ARENA_API_TOKEN" http://localhost:8080/api/v1/me
```

`GET /api/v1/me` returns the authenticated team, registered agent, active round, and whether the request used legacy team-token auth.

Lock a submitted agent to a replay/final-style round:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/rounds/1/agents/1/lock \
  -H "Authorization: Bearer dev-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"commit_sha":"abc123","docker_image":"team-01:final"}'
```

Replay-mode rounds require the authenticated agent to be locked to that round before it can heartbeat, submit decisions, submit orders, or cancel orders. Locked/replay round activation preflights every active team and fails if any active team lacks one active locked agent. Practice mode remains open to any active registered agent on the team unless the instructor explicitly enables locked-agent enforcement:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/rounds/1/require-locked-agents \
  -H "Authorization: Bearer dev-admin-token"
```

Changing a lock during an active round requires explicit confirmation:

```bash
just lock-agent 1 final-1 abc123 team-01:final replace_active_round_lock
```

Completed-round locks are immutable.

## Local and Exposed Deployment

`docker-compose.yml` is local-first. Backend, frontend, and Redis bind to `127.0.0.1` by default and read knobs from `.env`/`.env.example`. For a Tailscale, firewall-protected class-network host, or reverse-proxy setup, set strong secrets and use:

```bash
ARENA_ENV=exposed
ARENA_ADMIN_TOKEN=$(openssl rand -base64 32)
ARENA_AUDIT_SALT=$(openssl rand -base64 32)
ARENA_ALLOWED_ORIGINS=https://your-admin-host.example
ARENA_PUBLIC_TEAM_ACTIVITY=summary
ARENA_TRUST_PROXY_HEADERS=false
ARENA_RATE_LIMIT_ENABLED=true
ARENA_RATE_LIMIT_FAIL_CLOSED=true
just docker-up-exposed
```

Do not expose this app directly to the public internet. In exposed mode the backend refuses to start with `dev-admin-token`, a short admin token, a weak audit salt, disabled/fail-open rate limits, or wildcard CORS origins. Redis remains bound to localhost in the exposed override. Proxy headers are ignored by default; only enable `ARENA_TRUST_PROXY_HEADERS=true` with a tight `ARENA_TRUSTED_PROXY_CIDRS` allowlist when the backend sits behind a trusted reverse proxy.

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
- Redis-backed route rate limits for public reads, student reads, heartbeats, decisions, orders, admin routes, and auth failures.
- available simulated cash, including open buy-order reserves
- `require_reason`: `true`
- `require_estimated_probability`: `true`
- `allow_market_orders`: `false`
- probability and limit price ranges: `1..9999` bps

Failed checks create a rejected order when appropriate, create a risk event, append JSONL, and return a structured `400` response.

Route rate limits protect API availability and return `429` without creating competition artifacts. The risk `max_orders_per_minute` rule is a competition policy: it is DB-counted, creates rejected orders/risk events when breached, and contributes to execution quality.

## Accounting and Settlement

The bootcamp accounting model is average cost:

- Position quantity is simulated contract-cents.
- Average entry price is stored in bps.
- Buys update average entry price.
- Sells reduce quantity and realize PnL against average cost.
- Cash reflects initial balance, buys, sells, fees, and settlement payouts.
- Equity is cash plus mark-to-market exposure.

For settlement, resolved YES contracts pay `10000` bps on yes and `0` on no. Resolved NO contracts pay `10000` bps on no and `0` on yes. Settlement is idempotent and new student trades are rejected on resolved markets.

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
- If Redis is unavailable, the backend logs warnings and falls back to DB computation. Rate limiting fails open by default for local availability; set `ARENA_RATE_LIMIT_FAIL_CLOSED=true` when protection should take priority over availability.
- Use `just backup-sqlite` for an online SQLite backup through `VACUUM INTO`.
- Use `just compact-snapshots practice-1` to retain representative snapshots and reduce DB/export noise.
- For full recovery steps, see `docs/instructor-runbook.md`.

## More Docs

- `docs/student-quickstart.md`
- `docs/instructor-runbook.md`
- `BOOTCAMP.md`

## Roadmap

- Final replay rounds and historical replay adapter.
- Strategy report export and calibration charts.
- More instructor controls for allowlists and risk policy editing.
- Optional adapter wrapping `agent-next/polymarket-paper-trader`.
- Optional Kalshi Demo venue behind the same interface.
