# Student Quickstart

This arena is simulated/paper-trading only. Do not connect real-money exchange accounts, wallets, private keys, or production trading credentials to your agent.

## Environment

Set these variables before running an agent:

```bash
export ARENA_BASE_URL=http://localhost:8080
export ARENA_API_TOKEN=paa_agent_...
```

Your instructor gives you one registered agent token. Treat it like a password. The arena stores only a token hash and cannot print the token again later. Tokens normally start with `paa_agent_`.

## Run the Random Agent

```bash
cd examples/random-agent
ARENA_API_TOKEN=paa_agent_... mise exec -- go run .
```

The agent sends heartbeats, fetches allowed markets, and submits small random orders.

Python SDK option:

```bash
PYTHONPATH=sdk/python \
ARENA_BASE_URL=http://localhost:8080 \
ARENA_API_TOKEN=paa_agent_... \
mise exec -- python examples/python-random-agent/agent.py
```

Install the SDK for your own project with:

```bash
mise exec -- python -m pip install -e sdk/python
```

See `sdk/python/README.md` for SDK methods, models, and exceptions. See `docs/agent-contract.md` for the full student API contract.

## Submit a Heartbeat

```bash
curl -sS -X POST "$ARENA_BASE_URL/api/v1/heartbeat" \
  -H "Authorization: Bearer $ARENA_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"online","metadata":{"agent":"my-agent"}}'
```

Send a heartbeat every 10-30 seconds while your agent is running.

## Fetch Markets

```bash
curl -sS "$ARENA_BASE_URL/api/v1/markets"
```

Only markets allowlisted for the active round are returned. Public market responses omit instructor-only metadata, hidden true probabilities, price paths, and final outcomes.

## Check Your Agent Identity

```bash
curl -sS "$ARENA_BASE_URL/api/v1/me" \
  -H "Authorization: Bearer $ARENA_API_TOKEN"
```

This returns your team, registered agent, active round, and whether legacy team-token auth was used.

## Submit an Order

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

`POST /api/v1/orders` creates a decision and an order unless your payload references a prior decision. In the fake venue, marketable valid orders fill immediately at the simulated price. Nonmarketable limit orders can remain open and fill later if the simulated price path crosses your limit.

## Expected Decision Payload

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

Use integer cents for money. Use basis points for prices and probabilities, where `10000 = 100%`.

## Simulated Accounting

The arena uses average-cost accounting:

- Money is integer cents.
- Prices/probabilities are bps.
- Position quantity is simulated contract-cents.
- Buys update your average entry price.
- Sells realize PnL against average cost.
- Resolved YES pays `10000` bps on yes and `0` on no.
- Resolved NO pays `10000` bps on no and `0` on yes.

Resolved markets reject new orders.

## Rate Limits

Default local rate limits:

- Orders: 10 per minute per agent.
- Decisions: 30 per minute per agent.
- Heartbeats: 12 per minute per agent.
- Student reads: 120 per minute per agent.

If Redis is unavailable, the backend still runs. Local mode fails route rate limits open by default, while the DB-backed order-count risk check still applies to orders. Route limits protect API availability and return `429`; the order-count risk rule is a competition rule and can create rejected orders/risk events.

## Risk Limits

Default limits:

- Max order value: `50000` cents.
- Max position per market: `100000` cents.
- Max total exposure: `400000` cents.
- Max open orders: `20`.
- Buy orders must fit available simulated cash after reserving open buy orders.
- Estimated probability is required.
- Reason text is required.
- Market orders are disabled; include `limit_price_bps`.
- Probability and limit price must be between `1` and `9999` basis points.

Rejected orders appear in your activity and count against execution quality.

Public team pages show summary-only activity during active competition rounds. Your instructor can inspect full details from the admin console, and may enable completed-round postmortems after scoring. Even when full public postmortems are enabled, active rounds remain summary/redacted.

## Common Errors

- `missing_token`: add `Authorization: Bearer $ARENA_API_TOKEN`.
- `invalid_token`: check that you are using the `paa_agent_...` token printed when your agent was created.
- `inactive_team`: your instructor paused your team.
- `team_not_enrolled`: your team is not enrolled in the active round; ask the instructor to enroll it.
- `round_team_not_active`: your team enrollment is paused or withdrawn for this round.
- `paused_agent`: your instructor paused this registered agent; heartbeats and reads may still work, but trading is blocked.
- `revoked_agent`: the token was revoked and cannot be used.
- `rate_limit_exceeded`: slow down the request loop.
- `round_agent_lock_required`: the round requires locked registered agents; use your official agent token.
- `agent_not_locked_for_round`: your token is valid, but this agent is not the locked submission for the round.
- `no_active_round`: wait for the instructor to activate a round.
- `paused_round`: the instructor paused the round.
- `invalid_market`: the market is not allowlisted for the active round.
- `market_not_open`: the market is resolved or not accepting simulated orders.
- `insufficient_cash`: reduce order size or cancel open buy orders.
- `order_not_in_active_round`: only active-round open orders can be canceled.
- `order_agent_mismatch`: locked-round orders can only be canceled by the agent that created them.
- `risk_limit_exceeded`: inspect the response message and reduce size, add probability/reason, or slow down.
- `venue_unavailable`: retry later; the simulated venue adapter returned an error.

Errors use this shape:

```json
{
  "error": {
    "code": "risk_limit_exceeded",
    "message": "order amount exceeds max_order_value_cents",
    "details": {}
  }
}
```
