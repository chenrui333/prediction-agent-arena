from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path
from typing import Any

try:
    from arena_client import ArenaAPIError, ArenaClient, Market, RiskRejectedError, clamp_bps, price_for_outcome
except (ImportError, ModuleNotFoundError):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "sdk" / "python"))
    from arena_client import ArenaAPIError, ArenaClient, Market, RiskRejectedError, clamp_bps, price_for_outcome


def main() -> None:
    client = ArenaClient.from_env(max_retries=2)
    identity = client.me()
    agent_slug = identity.agent.slug if identity.agent else "legacy"
    print(f"anthropic-template starting team={identity.team.slug} agent={agent_slug}", flush=True)

    while True:
        try:
            client.heartbeat(metadata={"agent": "anthropic-agents-template"})
            markets = [market for market in client.markets().markets if market.status == "open"]
            if not markets:
                print("no open markets", flush=True)
                time.sleep(10)
                continue

            market = markets[0]
            decision = estimate_probability(market)
            result = client.order(
                market_id=market.id,
                outcome=decision["outcome"],
                action="buy",
                amount_cents=5000,
                limit_price_bps=decision["limit_price_bps"],
                estimated_probability_bps=decision["estimated_probability_bps"],
                confidence=decision["confidence"],
                reason=decision["reason"],
            )
            print(f"order market={market.slug} outcome={decision['outcome']} status={result.order.status}", flush=True)
        except RiskRejectedError as err:
            print(f"risk_rejected code={err.code} message={err.message}", flush=True)
        except ArenaAPIError as err:
            print(f"arena_error status={err.status} code={err.code} message={err.message}", flush=True)
        time.sleep(float(os.environ.get("ARENA_AGENT_INTERVAL_SECONDS", "15")))


def estimate_probability(market: Market) -> dict[str, Any]:
    llm_decision = estimate_with_anthropic_if_configured(market)
    if llm_decision is not None:
        return llm_decision
    return heuristic_decision(market)


def heuristic_decision(market: Market) -> dict[str, Any]:
    estimate = clamp_bps(market.yes_price_bps + 400)
    return {
        "outcome": "yes",
        "limit_price_bps": market.yes_price_bps,
        "estimated_probability_bps": estimate,
        "confidence": "low",
        "reason": "Fallback heuristic adds a small edge to the current YES price.",
    }


def estimate_with_anthropic_if_configured(market: Market) -> dict[str, Any] | None:
    if not os.environ.get("ANTHROPIC_API_KEY"):
        return None
    model = os.environ.get("ANTHROPIC_MODEL")
    if not model:
        return None
    try:
        from anthropic import Anthropic
    except ImportError:
        return None

    client = Anthropic()
    prompt = (
        "You are helping a simulated bootcamp prediction-market agent. "
        "Return only a JSON object with keys outcome, estimated_probability_bps, confidence, and reason. "
        "Outcome must be yes or no. estimated_probability_bps must be an integer from 1 to 9999. "
        "confidence must be low, medium, or high. Do not claim certainty. "
        "Do not mention real-money trading.\n\n"
        f"Market title: {market.title}\n"
        f"Category: {market.category}\n"
        f"YES price bps: {market.yes_price_bps}\n"
        f"NO price bps: {market.no_price_bps}\n"
    )
    try:
        message = client.messages.create(
            model=model,
            max_tokens=300,
            messages=[{"role": "user", "content": prompt}],
        )
    except Exception as err:
        print(f"anthropic_unavailable fallback=true error={type(err).__name__}", flush=True)
        return None

    raw_text = message_text(message)
    try:
        parsed = json.loads(raw_text)
    except json.JSONDecodeError:
        return None

    outcome = parsed.get("outcome", "yes")
    if outcome not in {"yes", "no"}:
        outcome = "yes"
    try:
        estimate = clamp_bps(int(parsed.get("estimated_probability_bps", market.yes_price_bps)))
    except (TypeError, ValueError):
        return None
    return {
        "outcome": outcome,
        "limit_price_bps": price_for_outcome(market, outcome),
        "estimated_probability_bps": estimate,
        "confidence": parsed.get("confidence", "low") if parsed.get("confidence") in {"low", "medium", "high"} else "low",
        "reason": str(parsed.get("reason", "Claude produced a conservative simulated paper-trading estimate."))[:240],
    }


def message_text(message: Any) -> str:
    parts = []
    for block in getattr(message, "content", []):
        text = getattr(block, "text", None)
        if isinstance(text, str):
            parts.append(text)
        elif isinstance(block, dict) and isinstance(block.get("text"), str):
            parts.append(block["text"])
    return "".join(parts).strip()


if __name__ == "__main__":
    main()
