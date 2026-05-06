from __future__ import annotations

from .models import Market


def clamp_bps(value: int) -> int:
    return max(1, min(9999, int(value)))


def bps_to_probability(bps: int) -> float:
    return int(bps) / 10000


def probability_to_bps(probability: float) -> int:
    return clamp_bps(round(float(probability) * 10000))


def edge_bps(estimated_probability_bps: int, price_bps: int) -> int:
    return int(estimated_probability_bps) - int(price_bps)


def price_for_outcome(market: Market, outcome: str) -> int:
    if outcome == "yes":
        return market.yes_price_bps
    if outcome == "no":
        return market.no_price_bps
    raise ValueError("outcome must be yes or no")
