# Agent Contract

This document defines the student-facing HTTP contract for agents. The API is local/simulated only and must not be used for real-money trading.

## Required Environment Variables

Agents should read:

```bash
export ARENA_BASE_URL=http://localhost:8080
export ARENA_API_TOKEN=paa_agent_...
```

Optional:

```bash
export ARENA_TIMEOUT_SECONDS=10
export ARENA_AGENT_INTERVAL_SECONDS=10
export OPENAI_MODEL=<your-available-openai-model>
export ANTHROPIC_MODEL=<your-available-claude-model>
```

Use registered agent tokens that start with `paa_agent_`. Do not hardcode tokens in source control, notebooks, screenshots, or reports.

## Student Endpoints

Authenticated with:

```http
Authorization: Bearer <paa_agent_token>
```

Available endpoints:

- `GET /api/v1/me`
- `GET /api/v1/markets`
- `GET /api/v1/portfolio`
- `POST /api/v1/heartbeat`
- `POST /api/v1/decisions`
- `POST /api/v1/orders`
- `POST /api/v1/orders/{order_id}/cancel`
- `GET /api/v1/fills`
- `GET /api/v1/leaderboard`

The student SDK intentionally does not include admin methods.

## Local Development Workflow

Install the thin Python SDK in editable mode:

```bash
mise exec -- python -m pip install -e sdk/python
```

Or run from the repo without installing:

```bash
PYTHONPATH=sdk/python ARENA_API_TOKEN=paa_agent_... mise exec -- python examples/python-random-agent/agent.py
```

Example env files:

```bash
cp examples/.env.example examples/.env
```

The `examples/` directory ignores `.env`, logs, and access/export artifacts. Do not commit real arena tokens or provider API keys.

Minimal SDK check:

```python
from arena_client import ArenaClient

client = ArenaClient.from_env()
identity = client.me()
print(identity.team.slug, identity.agent.slug if identity.agent else "legacy")
```

Optional provider-assisted templates:

```bash
PYTHONPATH=sdk/python ARENA_API_TOKEN=paa_agent_... mise exec -- python examples/openai-agents-template/agent.py
PYTHONPATH=sdk/python ARENA_API_TOKEN=paa_agent_... mise exec -- python examples/anthropic-agents-template/agent.py
```

The GPT/OpenAI template requires `OPENAI_API_KEY`, `OPENAI_MODEL`, and the optional `openai` package before it calls OpenAI. The Claude/Anthropic template requires `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`, and the optional `anthropic` package before it calls Claude. If provider configuration is missing, both templates use a local heuristic.

Equivalent curl check:

```bash
curl -sS "$ARENA_BASE_URL/api/v1/me" \
  -H "Authorization: Bearer $ARENA_API_TOKEN"
```

## Response And Error Shape

Successful responses are JSON objects.

Structured errors use:

```json
{
  "error": {
    "code": "risk_limit_exceeded",
    "message": "Order exceeds max_order_value_cents",
    "details": {
      "max_order_value_cents": 50000
    }
  }
}
```

Common status codes:

- `400`: malformed request or risk rejection.
- `401`: missing or invalid token.
- `403`: inactive team, paused/revoked agent, or locked-round mismatch.
- `404`: no active round, invalid market, or missing resource.
- `409`: paused round or round/order state conflict.
- `429`: route rate limit exceeded.
- `502`: optional venue unavailable.

The Python SDK raises structured exceptions for `401`, `403`, `409`, `429`, and risk rejections.

The SDK supports Python 3.11+ and has no runtime dependencies outside the Python standard library. Optional GPT/Claude examples may use provider SDKs, but those are example-local dependencies and are not required by `arena_client`.

SDK utility helpers are available for common unit handling: `clamp_bps`, `bps_to_probability`, `probability_to_bps`, `edge_bps`, and `price_for_outcome`.

Optional retry settings:

- `ARENA_MAX_RETRIES=2`
- `ARENA_RETRY_BACKOFF_SECONDS=1.0`

The SDK only retries network errors, request timeouts, HTTP `502`/`503`/`504`, and HTTP `429` when `Retry-After` is present. Retries apply to `GET` requests and heartbeat posts only. The SDK does not retry order, decision, or cancel-order posts because those mutations are not idempotent. It also does not retry risk rejections, auth errors, forbidden requests, or state conflicts.

## Identity

Request:

```bash
curl -sS "$ARENA_BASE_URL/api/v1/me" \
  -H "Authorization: Bearer $ARENA_API_TOKEN"
```

Response:

```json
{
  "team": {
    "id": 1,
    "slug": "team-01",
    "name": "Team 01",
    "is_active": true
  },
  "agent": {
    "id": 7,
    "team_id": 1,
    "team_slug": "team-01",
    "slug": "default",
    "name": "Team 01 Default Agent",
    "status": "active",
    "kind": "student"
  },
  "active_round": {
    "id": 1,
    "slug": "practice-1",
    "name": "Practice 1",
    "mode": "practice",
    "status": "active",
    "require_locked_agents": false,
    "initial_balance_cents": 1000000
  },
  "legacy_team_auth": false
}
```

## Heartbeat

Send a heartbeat every 5-30 seconds while your agent is running.

```bash
curl -sS -X POST "$ARENA_BASE_URL/api/v1/heartbeat" \
  -H "Authorization: Bearer $ARENA_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"online","metadata":{"agent":"my-agent","version":"dev"}}'
```

SDK:

```python
client.heartbeat(status="online", metadata={"agent": "my-agent"})
```

## Fetch Markets

```bash
curl -sS "$ARENA_BASE_URL/api/v1/markets"
```

SDK:

```python
markets = client.markets().markets
```

Public markets include current simulated prices but do not include private `metadata_json`.

## Fetch Portfolio

```bash
curl -sS "$ARENA_BASE_URL/api/v1/portfolio" \
  -H "Authorization: Bearer $ARENA_API_TOKEN"
```

SDK:

```python
portfolio = client.portfolio().portfolio
print(portfolio.cash_cents, portfolio.equity_cents)
```

## Decision Payload

`POST /api/v1/decisions` records a forecast without necessarily placing an order.

```json
{
  "market_id": 1,
  "outcome": "yes",
  "action": "buy",
  "amount_cents": 10000,
  "limit_price_bps": 5700,
  "estimated_probability_bps": 6400,
  "confidence": "medium",
  "reason": "My estimate is above the market implied probability."
}
```

SDK:

```python
client.decision(
    market_id=1,
    outcome="yes",
    action="buy",
    amount_cents=10000,
    limit_price_bps=5700,
    estimated_probability_bps=6400,
    confidence="medium",
    reason="My estimate is above the market implied probability.",
)
```

## Order Payload

`POST /api/v1/orders` accepts the same core fields and creates both a decision and an order.

```bash
curl -sS -X POST "$ARENA_BASE_URL/api/v1/orders" \
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

SDK:

```python
from arena_client import RiskRejectedError

try:
    result = client.order(
        market_id=1,
        outcome="yes",
        action="buy",
        amount_cents=10000,
        limit_price_bps=5700,
        estimated_probability_bps=6400,
        confidence="medium",
        reason="My estimate is above the market implied probability.",
    )
    print(result.order.status)
except RiskRejectedError as err:
    print(err.code, err.message, err.details)
```

## Cancel Order

Only submitted/open orders in the active round can be canceled by the owning team. Locked rounds also require the same locked agent that created the order.

```python
client.cancel_order(order_id=123)
```

## Fills And Leaderboard

```python
fills = client.fills()
leaderboard = client.leaderboard()
```

## Common Errors And Fixes

- `missing_token`: set `ARENA_API_TOKEN`.
- `invalid_token`: copy the one-time `paa_agent_...` token exactly.
- `inactive_team`: ask the instructor to resume the team.
- `inactive_agent`: ask the instructor to resume or rotate the agent.
- `no_active_round`: wait for the instructor to activate a round.
- `round_paused`: the instructor paused the round.
- `team_not_enrolled`: ask the instructor to enroll your team in the active round.
- `invalid_market`: refresh `/markets`; the market may not be allowlisted for the round.
- `malformed_probability`: keep `estimated_probability_bps` between `1` and `9999`.
- `amount_too_large`: reduce `amount_cents`.
- `insufficient_cash`: reduce order size or sell exposure.
- `max_open_orders_exceeded`: cancel stale open orders.
- `rate_limit_exceeded`: slow your loop down and add backoff.
- `venue_unavailable`: retry later; do not loop aggressively.

## Rate Limits

Default local limits are suitable for classroom agents:

- orders: `10/minute`
- decisions: `30/minute`
- heartbeat: `12/minute`
- authenticated reads: `120/minute`
- public reads: `120/minute`

Use a low-frequency loop. A good starting point is one heartbeat every 10 seconds and at most one order attempt every 10-30 seconds.

## Risk Limits

Default policy:

- max order value: `50000` cents
- max position per market: `100000` cents
- max total exposure: `400000` cents
- max open orders: `20`
- reason required
- estimated probability required
- market orders disabled

Treat risk rejections as feedback. Your agent should catch them, log the structured code/details, and back off.
