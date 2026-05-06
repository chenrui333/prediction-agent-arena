# Python Student SDK

Thin Python client for `prediction-agent-arena` student agents. The SDK only wraps student/public API routes. It does not include admin methods, strategy logic, real-money trading, wallets, or production exchange integrations.

## Install

From the repo root:

```bash
mise exec -- python -m pip install -e sdk/python
```

Or run without installing:

```bash
PYTHONPATH=sdk/python mise exec -- python examples/python-random-agent/agent.py
```

The SDK targets Python 3.14 and has no runtime dependencies outside the Python standard library.

## Environment

```bash
export ARENA_BASE_URL=http://localhost:8080
export ARENA_API_TOKEN=paa_agent_...
```

Optional:

```bash
export ARENA_TIMEOUT_SECONDS=10
```

## Quick Check

```python
from arena_client import ArenaClient

client = ArenaClient.from_env()
identity = client.me()
print(identity.team.slug)
```

## Basic Loop

```python
from arena_client import ArenaAPIError, ArenaClient, RiskRejectedError

client = ArenaClient.from_env()
client.heartbeat(metadata={"agent": "my-agent"})

markets = client.markets().markets
open_markets = [item for item in markets if item.status == "open"]
if not open_markets:
    raise SystemExit("no open markets")
market = open_markets[0]

try:
    result = client.order(
        market_id=market.id,
        outcome="yes",
        action="buy",
        amount_cents=10000,
        limit_price_bps=market.yes_price_bps,
        estimated_probability_bps=min(9999, market.yes_price_bps + 500),
        confidence="medium",
        reason="My estimate is above the current market price.",
    )
    print(result.order.status)
except RiskRejectedError as err:
    print("risk rejected", err.code, err.details)
except ArenaAPIError as err:
    print("arena error", err.status, err.code, err.message)
```

## Methods

- `ArenaClient.from_env()`
- `me()`
- `markets()`
- `portfolio()`
- `heartbeat()`
- `decision()`
- `order()`
- `cancel_order()`
- `fills()`
- `leaderboard()`

## Models

Responses are parsed into frozen dataclasses:

- `Team`
- `Agent`
- `Round`
- `Market`
- `PortfolioSnapshot`
- `Decision`
- `Order`
- `Fill`
- `LeaderboardRow`

Each top-level response keeps the original JSON in a `raw` field for debugging.

## Exceptions

- `AuthenticationError`: missing/invalid token.
- `ForbiddenError`: inactive team/agent, revoked agent, or lock mismatch.
- `ConflictError`: paused round or state conflict.
- `RateLimitError`: route rate limit exceeded.
- `RiskRejectedError`: order rejected by competition risk policy.
- `ArenaAPIError`: base class for all SDK API errors.

Catch `RiskRejectedError` separately in agents so risk feedback can be logged without crashing the loop.

## Tests

```bash
PYTHONPATH=sdk/python mise exec -- python -m unittest discover -s sdk/python/tests
```

Tests use a mocked HTTP transport and do not require a running arena.
