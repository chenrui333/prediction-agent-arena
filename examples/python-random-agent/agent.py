from __future__ import annotations

import random
import sys
import time
from pathlib import Path

try:
    from arena_client import ArenaAPIError, ArenaClient, RiskRejectedError
except ModuleNotFoundError:
    sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "sdk" / "python"))
    from arena_client import ArenaAPIError, ArenaClient, RiskRejectedError


def main() -> None:
    client = ArenaClient.from_env()
    identity = client.me()
    agent_slug = identity.agent.slug if identity.agent else "legacy"
    print(f"random-agent starting team={identity.team.slug} agent={agent_slug}", flush=True)

    while True:
        try:
            client.heartbeat(metadata={"agent": "python-random-agent"})
            markets = [market for market in client.markets().markets if market.status == "open"]
            if not markets:
                print("no open markets", flush=True)
                time.sleep(10)
                continue

            market = random.choice(markets)
            outcome = random.choice(["yes", "no"])
            action = random.choice(["buy", "sell"])
            market_price = market.yes_price_bps if outcome == "yes" else market.no_price_bps
            estimate = clamp_bps(market_price + random.randint(-900, 900))
            limit_price = clamp_bps(market_price + random.randint(-150, 150))

            result = client.order(
                market_id=market.id,
                outcome=outcome,
                action=action,
                amount_cents=5000,
                limit_price_bps=limit_price,
                estimated_probability_bps=estimate,
                confidence="low",
                reason=f"Random baseline sampled {estimate} bps for {outcome}.",
            )
            print(f"order market={market.slug} action={action} {outcome} status={result.order.status}", flush=True)
        except RiskRejectedError as err:
            print(f"risk_rejected code={err.code} message={err.message}", flush=True)
        except ArenaAPIError as err:
            print(f"arena_error status={err.status} code={err.code} message={err.message}", flush=True)
        time.sleep(random.uniform(8, 14))


def clamp_bps(value: int) -> int:
    return max(1, min(9999, value))


if __name__ == "__main__":
    main()
