# prediction-agent-arena

Local simulated prediction-market agent arena for cohorts, competitions, and agent evaluations. Participants run registered trading agents from their laptops and call an operator-run API. The arena handles teams, rounds, market allowlists, agent tokens, heartbeats, decisions, orders, risk checks, fake fills, open-order fills, settlement, portfolio snapshots, scoring, Redis-cached leaderboards, JSONL/CSV export, and a simple live UI.

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

- Go `1.26.3`
- Node `24.15.0`
- Python `3.14.4`
- just `1.50.0`

Install tools with:

```bash
mise install
```

Then use `just` for local workflows.

## Terminology

A participant is a person or group entering the arena. A team is the leaderboard identity for one participant group. An agent is the executable bot that authenticates with an agent token and interacts with the arena API. A submission is a specific registered agent version locked to a round. An operator is the person administering rounds, markets, access tokens, and exports.

## Architecture

- `backend/`: Go API and worker using `net/http`, `chi`, `database/sql`, `modernc.org/sqlite`, `go-redis`, and `slog`.
- `frontend/`: Next.js App Router + TypeScript dashboard.
- `examples/`: Go and Python example agents.
- `sdk/python/`: thin Python Arena SDK for agent development.
- `agent-skills/`: short participant guides for building and debugging agents.
- `scripts/`: thin Go-backed seed and export helpers.
- `data/arena.db`: local SQLite DB mounted into containers.
- `logs/{round_slug}/{team_slug}.events.jsonl`: append-only arena event logs.
- `exports/{round_slug}/`: leaderboard CSV, score JSONL, per-team bundles, and grading reports.

SQLite is the source of truth. Redis is only a cache and rate-limit helper. Money is stored as integer cents; prices and probabilities are basis points where `10000 = 100%`. Positions use simulated contract-cents and average-cost accounting. Agent API credentials are registered agent tokens (`paa_agent_...`) stored only as hashes. Legacy team-token auth is disabled by default and can be enabled only with `ARENA_LEGACY_TEAM_TOKEN_AUTH=true`.

Public team pages default to summary mode during active rounds so teams cannot inspect each other's reasoning, orders, fills, or risk events mid-competition. Operators can use the admin API/UI for full activity. `ARENA_PUBLIC_TEAM_ACTIVITY=full` means full public postmortems after a round is completed; active rounds remain summary/redacted.

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
- Onboarding: http://localhost:3000/onboard
- Agent launchpad: http://localhost:3000/agent
- Backend health: http://localhost:8080/health

`just seed` creates 10 demo teams, one active round (`practice-1`), fake markets with deterministic price paths, enrolls the demo teams in the round, and creates one default registered agent per team. It prints newly generated agent tokens once and writes matching one-time access packets under `exports/access/`. Existing tokens are never reprinted.

## Practice And Contest Flow

The hosted arena uses one onboarding hub and two signup phases:

- Practice signup is shared privately in Discord and can stay open for ad-hoc setup, synthetic-data testing, and agent iteration.
- Contest signup is a timed Discord window with a close time and official agent lock deadline.
- Practice leaderboard scores are informal and may be reset.
- Official contest results come from a separate contest/evaluation round after it is completed, frozen, and exported.
- Keep Discord invites, signup links, admin tokens, and agent tokens out of tracked files and public support screenshots.

See docs/onboarding.md for Discord templates, operator preflight checks, and the practice/contest runbook.

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

Python SDK quickstart:

```bash
mise exec -- python -m pip install -e sdk/python
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
ARENA_MAX_RETRIES=2 \
mise exec -- python examples/python-random-agent/agent.py
```

Or without installing:

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
ARENA_MAX_RETRIES=2 \
mise exec -- python examples/python-random-agent/agent.py
```

Optional OpenAI-assisted template:

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/openai-agents-template/agent.py
```

Set `OPENAI_API_KEY`, `OPENAI_MODEL`, and install `openai` only if you want the template to call OpenAI for bounded probability estimation. Without both provider env vars, the template falls back to a local heuristic.

Optional Claude-assisted template:

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/anthropic-agents-template/agent.py
```

Set `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`, and install `anthropic` only if you want the template to call Claude for bounded probability estimation. Provider model names are intentionally explicit because participant account access can vary.

Agent example token hygiene:

```bash
cp examples/.env.example examples/.env
```

The example directory ignores `.env`, logs, and access/export artifacts.

The agent launchpad at http://localhost:3000/agent verifies `/api/v1/me` with a pasted agent token, keeps optional token memory scoped to the browser tab, and shows copyable curl/SDK commands. The leaderboard and evaluation views refresh automatically every 5 seconds. `/student` redirects to `/agent` for one transitional cycle.

## Arena SDK

The Python SDK is intentionally thin. It supports Python 3.11+, wraps agent/public routes only, and does not include strategy, admin methods, wallets, or production exchange behavior.

```python
from arena_client import ArenaClient, RiskRejectedError, clamp_bps, price_for_outcome

client = ArenaClient.from_env(max_retries=2)
print(client.me().team.slug)

market = client.markets().markets[0]
try:
    result = client.order(
        market_id=market.id,
        outcome="yes",
        action="buy",
        amount_cents=10000,
        limit_price_bps=price_for_outcome(market, "yes"),
        estimated_probability_bps=clamp_bps(price_for_outcome(market, "yes") + 500),
        confidence="medium",
        reason="My estimate is above the current YES price.",
    )
    print(result.order.status)
except RiskRejectedError as err:
    print(err.code, err.details)
```

See `sdk/python/README.md` and `docs/agent-contract.md` for the complete SDK and endpoint contract.

Optional SDK retries apply only to safe reads and heartbeat posts. Order, decision, and cancel-order posts are not retried because duplicate mutation attempts can create duplicate simulated orders.

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
just create-agent-access team-11 default "Team 11 Default Agent"
just create-round practice-2 "Practice Round 2"
just enroll-round-team team-11 practice-2
just list-round-teams practice-2
just activate-round practice-2
just require-locked-agents practice-2
just allow-unlocked-agents practice-2
just pause-round-team team-11 practice-2
just resume-round-team team-11 practice-2
just withdraw-round-team team-11 practice-2
just pause-team team-03
just resume-team team-03
just pause-agent 1
just resume-agent 1
just revoke-agent 1
just lock-agent 1 practice-1 abc123 team-01:evaluation
just list-round-agents practice-1
just reset-team team-03
just rotate-team-token team-03
just rotate-agent-token 1
just rotate-agent-token-access 1
just settle-round practice-1 settle_active_round true
just compact-snapshots practice-1
just compact-audit 14d
just backup-sqlite
just health
just freeze-leaderboard practice-1
just print-active-round
just print-team-tokens
```

Rounds have explicit team enrollment. A team must be enrolled and active in the active round before its agents can heartbeat, read portfolio/fills, submit decisions/orders, or cancel orders. `just reset-team` is round-scoped by default. Use `just reset-team-all-rounds team-03` only when you intentionally want to delete that team history across every round. `just print-team-tokens` intentionally does not dump existing secrets. Tokens are shown only when a team or agent is created or rotated. Agent tokens are the normal participant credential; team tokens exist only for operator workflows and optional legacy compatibility.

## Admin UI

Open http://localhost:3000/admin and enter the admin token from `.env` (`dev-admin-token` by default for local-only mode). The token is stored only for the current browser tab/session and can be cleared with the `Forget token` button. The page shows active round state, readiness checks, teams, agents, rounds, last heartbeat, equity, trade count, risk rejections, exposure, and health state. It exposes pause/resume/reset team controls, round enrollment controls, create/pause/resume/revoke/rotate agent controls, round lifecycle controls, settlement, snapshot compaction, leaderboard freeze, and export.

## Frontend Pages

- /onboard: practice and timed-contest onboarding hub with active-round status, Discord guidance, and safe local agent commands.
- `/`: arena overview, active round summary, markets, and links to agent/admin/leaderboard pages.
- `/agent`: local agent launchpad for verifying an agent token and copying SDK/curl commands. It uses in-memory or tab-scoped session storage, not `localStorage`.
- `/student`: compatibility redirect to `/agent`.
- `/leaderboard`: projector-readable leaderboard with 5-second refresh and a last-updated timestamp.
- `/leaderboard/evaluation`: larger evaluation-round projector view with top-three cards and fewer columns.
- `/leaderboard/finals`: compatibility redirect to `/leaderboard/evaluation`.
- `/teams/{teamSlug}`: public team summary, portfolio values, trade/risk counts, and last heartbeat. Detailed decisions/orders/fills/risk events are redacted during active competition rounds.
- `/admin`: minimal operator console backed by the Go admin API, including a round readiness panel for health, enrollment, markets, and locked-agent checks.

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
  -d '{"slug":"default","name":"Team 11 Default Agent","kind":"agent"}'
```

Agent mutation and portfolio routes require a registered agent token:

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

`GET /api/v1/me` returns the authenticated team, registered agent, active round, and whether the request used legacy team-token auth. Public market endpoints intentionally omit `metadata_json` and simulation internals; keep operator-only notes, true probabilities, and final outcomes in admin-only market metadata.

Lock a submitted agent to a replay/evaluation-style round:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/rounds/1/agents/1/lock \
  -H "Authorization: Bearer dev-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"commit_sha":"abc123","docker_image":"team-01:evaluation"}'
```

Replay-mode rounds require the authenticated agent to be locked to that round before it can heartbeat, submit decisions, submit orders, or cancel orders. Round activation requires at least one active enrolled team. Locked/replay round activation preflights active enrolled teams and fails if any active enrolled team lacks one active locked agent. Practice mode remains open to any active registered agent on an active enrolled team unless the operator explicitly enables locked-agent enforcement:

```bash
curl -sS -X POST http://localhost:8080/api/v1/admin/rounds/1/require-locked-agents \
  -H "Authorization: Bearer dev-admin-token"
```

Changing a lock during an active round requires explicit confirmation:

```bash
just lock-agent 1 eval-1 abc123 team-01:evaluation replace_active_round_lock
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

Do not expose this app directly to the public internet. In exposed mode the backend refuses to start with `dev-admin-token`, a short admin token, a weak audit salt, disabled/fail-open rate limits, legacy team-token auth, or wildcard CORS origins. Redis remains bound to localhost in the exposed override. Proxy headers are ignored by default; only enable `ARENA_TRUST_PROXY_HEADERS=true` with a tight `ARENA_TRUSTED_PROXY_CIDRS` allowlist when the backend sits behind a trusted reverse proxy.

For always-on battle testing, use [docs/deployment-hosting.md](docs/deployment-hosting.md) to choose between self-hosting, the recommended Fly.io-style full-stack hosted path, and optional Vercel frontend-only hosting.

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
- Redis-backed route rate limits for public reads, agent reads, heartbeats, decisions, orders, admin routes, and auth failures.
- available simulated cash, including open buy-order reserves
- `require_reason`: `true`
- `require_estimated_probability`: `true`
- `allow_market_orders`: `false`
- probability and limit price ranges: `1..9999` bps

Failed checks create a rejected order when appropriate, create a risk event, append JSONL, and return a structured `400` response.

Route rate limits protect API availability and return `429` without creating competition artifacts. The risk `max_orders_per_minute` rule is a competition policy: it is DB-counted, creates rejected orders/risk events when breached, and contributes to execution quality.

## Accounting and Settlement

The arena accounting model is average cost:

- Position quantity is simulated contract-cents.
- Average entry price is stored in bps.
- Buys update average entry price.
- Sells reduce quantity and realize PnL against average cost.
- Cash reflects initial balance, buys, sells, fees, and settlement payouts.
- Equity is cash plus mark-to-market exposure.

For settlement, resolved YES contracts pay `10000` bps on yes and `0` on no. Resolved NO contracts pay `10000` bps on no and `0` on yes. Settlement is idempotent and new agent trades are rejected on resolved markets. The settle API rejects unresolved round markets; settling an active round requires `confirm=settle_active_round`, and `complete_after_settlement=true` can complete the round after a successful settlement pass.

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
- For full recovery steps, see `docs/operator-runbook.md`.

## More Docs

- docs/onboarding.md
- `docs/participant-quickstart.md`
- `docs/game-logic.md`
- `docs/agent-contract.md`
- `docs/operator-runbook.md`
- `sdk/python/README.md`
- `agent-skills/build-basic-agent.md`
- `agent-skills/build-llm-agent.md`
- `agent-skills/debug-risk-rejections.md`
- `agent-skills/evaluation-round-checklist.md`
- `agent-skills/write-evaluation-report.md`
- `BOOTCAMP.md`

## Roadmap

- Evaluation replay rounds and historical replay adapter.
- Strategy report export and calibration charts.
- More operator controls for allowlists and risk policy editing.
- Optional adapter wrapping `agent-next/polymarket-paper-trader`.
- Optional Kalshi Demo venue behind the same interface.
