from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path
from typing import Any

try:
    from arena_client import ArenaAPIError, ArenaClient, Market, RiskRejectedError
except ModuleNotFoundError:
    sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "sdk" / "python"))
    from arena_client import ArenaAPIError, ArenaClient, Market, RiskRejectedError


def main() -> None:
    client = ArenaClient.from_env()
    identity = client.me()
    agent_slug = identity.agent.slug if identity.agent else "legacy"
    print(f"openai-template starting team={identity.team.slug} agent={agent_slug}", flush=True)

    while True:
        try:
            client.heartbeat(metadata={"agent": "openai-agents-template"})
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
    llm_decision = estimate_with_openai_if_configured(market)
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


def estimate_with_openai_if_configured(market: Market) -> dict[str, Any] | None:
    if not os.environ.get("OPENAI_API_KEY"):
        return None
    try:
        from openai import OpenAI
    except ImportError:
        return None

    client = OpenAI()
    model = os.environ.get("OPENAI_MODEL", "gpt-5.4-mini")
    prompt = (
        "You are helping a simulated bootcamp prediction-market agent. "
        "Return a conservative JSON decision for one paper-trading market. "
        "Do not claim certainty. Do not mention real-money trading.\n\n"
        f"Market title: {market.title}\n"
        f"Category: {market.category}\n"
        f"YES price bps: {market.yes_price_bps}\n"
        f"NO price bps: {market.no_price_bps}\n"
    )
    try:
        response = client.responses.create(
            model=model,
            input=prompt,
            text={
                "format": {
                    "type": "json_schema",
                    "name": "arena_decision",
                    "strict": True,
                    "schema": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "outcome": {"type": "string", "enum": ["yes", "no"]},
                            "estimated_probability_bps": {"type": "integer", "minimum": 1, "maximum": 9999},
                            "confidence": {"type": "string", "enum": ["low", "medium", "high"]},
                            "reason": {"type": "string", "minLength": 10, "maxLength": 240},
                        },
                        "required": ["outcome", "estimated_probability_bps", "confidence", "reason"],
                    },
                }
            },
        )
    except Exception as err:
        print(f"openai_unavailable fallback=true error={type(err).__name__}", flush=True)
        return None

    raw_text = getattr(response, "output_text", "")
    try:
        parsed = json.loads(raw_text)
    except json.JSONDecodeError:
        return None

    outcome = parsed.get("outcome", "yes")
    if outcome not in {"yes", "no"}:
        outcome = "yes"
    estimate = clamp_bps(int(parsed.get("estimated_probability_bps", market.yes_price_bps)))
    limit_price = market.yes_price_bps if outcome == "yes" else market.no_price_bps
    return {
        "outcome": outcome,
        "limit_price_bps": limit_price,
        "estimated_probability_bps": estimate,
        "confidence": parsed.get("confidence", "low") if parsed.get("confidence") in {"low", "medium", "high"} else "low",
        "reason": str(parsed.get("reason", "LLM produced a conservative simulated paper-trading estimate."))[:240],
    }


def clamp_bps(value: int) -> int:
    return max(1, min(9999, value))


if __name__ == "__main__":
    main()
