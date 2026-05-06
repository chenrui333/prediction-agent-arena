from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


def _string(data: dict[str, Any], key: str, default: str = "") -> str:
    value = data.get(key, default)
    return default if value is None else str(value)


def _int(data: dict[str, Any], key: str, default: int = 0) -> int:
    value = data.get(key, default)
    return default if value is None else int(value)


def _optional_int(data: dict[str, Any], key: str) -> int | None:
    value = data.get(key)
    return int(value) if value is not None else None


def _bool(data: dict[str, Any], key: str, default: bool = False) -> bool:
    value = data.get(key, default)
    return bool(value)


@dataclass(frozen=True)
class Team:
    id: int
    slug: str
    name: str
    is_active: bool = True

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Team":
        return cls(id=_int(data, "id"), slug=_string(data, "slug"), name=_string(data, "name"), is_active=_bool(data, "is_active", True))


@dataclass(frozen=True)
class Agent:
    id: int
    team_id: int
    slug: str
    name: str
    status: str
    kind: str
    team_slug: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Agent":
        return cls(
            id=_int(data, "id"),
            team_id=_int(data, "team_id"),
            team_slug=_string(data, "team_slug"),
            slug=_string(data, "slug"),
            name=_string(data, "name"),
            status=_string(data, "status"),
            kind=_string(data, "kind"),
        )


@dataclass(frozen=True)
class Round:
    id: int
    slug: str
    name: str
    mode: str
    status: str
    require_locked_agents: bool = False
    initial_balance_cents: int = 0

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Round":
        return cls(
            id=_int(data, "id"),
            slug=_string(data, "slug"),
            name=_string(data, "name"),
            mode=_string(data, "mode"),
            status=_string(data, "status"),
            require_locked_agents=_bool(data, "require_locked_agents"),
            initial_balance_cents=_int(data, "initial_balance_cents"),
        )


@dataclass(frozen=True)
class Market:
    id: int
    venue: str
    external_id: str
    slug: str
    title: str
    category: str
    status: str
    yes_price_bps: int
    no_price_bps: int

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Market":
        return cls(
            id=_int(data, "id"),
            venue=_string(data, "venue"),
            external_id=_string(data, "external_id"),
            slug=_string(data, "slug"),
            title=_string(data, "title"),
            category=_string(data, "category"),
            status=_string(data, "status"),
            yes_price_bps=_int(data, "yes_price_bps"),
            no_price_bps=_int(data, "no_price_bps"),
        )


@dataclass(frozen=True)
class MarketsResponse:
    round: Round | None
    markets: list[Market]
    raw: dict[str, Any] = field(repr=False, default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "MarketsResponse":
        round_data = data.get("round")
        return cls(
            round=Round.from_dict(round_data) if isinstance(round_data, dict) else None,
            markets=[Market.from_dict(item) for item in data.get("markets", [])],
            raw=data,
        )


@dataclass(frozen=True)
class MeResponse:
    team: Team
    agent: Agent | None
    active_round: Round | None
    legacy_team_auth: bool
    raw: dict[str, Any] = field(repr=False, default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "MeResponse":
        agent_data = data.get("agent")
        round_data = data.get("active_round")
        return cls(
            team=Team.from_dict(data.get("team", {})),
            agent=Agent.from_dict(agent_data) if isinstance(agent_data, dict) else None,
            active_round=Round.from_dict(round_data) if isinstance(round_data, dict) else None,
            legacy_team_auth=_bool(data, "legacy_team_auth"),
            raw=data,
        )


@dataclass(frozen=True)
class PortfolioSnapshot:
    cash_cents: int
    equity_cents: int
    realized_pnl_cents: int
    unrealized_pnl_cents: int
    gross_exposure_cents: int
    max_drawdown_bps: int
    created_at: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "PortfolioSnapshot":
        return cls(
            cash_cents=_int(data, "cash_cents"),
            equity_cents=_int(data, "equity_cents"),
            realized_pnl_cents=_int(data, "realized_pnl_cents"),
            unrealized_pnl_cents=_int(data, "unrealized_pnl_cents"),
            gross_exposure_cents=_int(data, "gross_exposure_cents"),
            max_drawdown_bps=_int(data, "max_drawdown_bps"),
            created_at=_string(data, "created_at"),
        )


@dataclass(frozen=True)
class PortfolioResponse:
    round: Round
    team: Team
    portfolio: PortfolioSnapshot
    raw: dict[str, Any] = field(repr=False, default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "PortfolioResponse":
        return cls(
            round=Round.from_dict(data.get("round", {})),
            team=Team.from_dict(data.get("team", {})),
            portfolio=PortfolioSnapshot.from_dict(data.get("portfolio", {})),
            raw=data,
        )


@dataclass(frozen=True)
class Decision:
    id: int
    round_id: int
    team_id: int
    market_id: int
    observed_price_bps: int
    edge_bps: int
    action: str
    outcome: str
    amount_cents: int
    confidence: str
    reason: str
    created_at: str = ""
    agent_id: int | None = None
    estimated_probability_bps: int | None = None

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Decision":
        estimated = data.get("estimated_probability_bps")
        agent_id = data.get("agent_id")
        return cls(
            id=_int(data, "id"),
            round_id=_int(data, "round_id"),
            team_id=_int(data, "team_id"),
            agent_id=int(agent_id) if agent_id is not None else None,
            market_id=_int(data, "market_id"),
            observed_price_bps=_int(data, "observed_price_bps"),
            estimated_probability_bps=int(estimated) if estimated is not None else None,
            edge_bps=_int(data, "edge_bps"),
            action=_string(data, "action"),
            outcome=_string(data, "outcome"),
            amount_cents=_int(data, "amount_cents"),
            confidence=_string(data, "confidence"),
            reason=_string(data, "reason"),
            created_at=_string(data, "created_at"),
        )


@dataclass(frozen=True)
class Order:
    id: int
    round_id: int
    team_id: int
    market_id: int
    action: str
    outcome: str
    amount_cents: int
    limit_price_bps: int
    status: str
    created_at: str = ""
    agent_id: int | None = None
    venue_order_id: str = ""
    rejection_reason: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Order":
        agent_id = data.get("agent_id")
        return cls(
            id=_int(data, "id"),
            round_id=_int(data, "round_id"),
            team_id=_int(data, "team_id"),
            agent_id=int(agent_id) if agent_id is not None else None,
            market_id=_int(data, "market_id"),
            venue_order_id=_string(data, "venue_order_id"),
            action=_string(data, "action"),
            outcome=_string(data, "outcome"),
            amount_cents=_int(data, "amount_cents"),
            limit_price_bps=_int(data, "limit_price_bps"),
            status=_string(data, "status"),
            rejection_reason=_string(data, "rejection_reason"),
            created_at=_string(data, "created_at"),
        )


@dataclass(frozen=True)
class Fill:
    id: int
    order_id: int
    market_id: int
    action: str
    outcome: str
    amount_cents: int
    fill_price_bps: int
    fee_cents: int
    slippage_bps: int
    created_at: str = ""
    round_id: int | None = None
    team_id: int | None = None
    agent_id: int | None = None

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Fill":
        return cls(
            id=_int(data, "id"),
            order_id=_int(data, "order_id"),
            market_id=_int(data, "market_id"),
            action=_string(data, "action"),
            outcome=_string(data, "outcome"),
            amount_cents=_int(data, "amount_cents"),
            fill_price_bps=_int(data, "fill_price_bps"),
            fee_cents=_int(data, "fee_cents"),
            slippage_bps=_int(data, "slippage_bps"),
            created_at=_string(data, "created_at"),
            round_id=_optional_int(data, "round_id"),
            team_id=_optional_int(data, "team_id"),
            agent_id=_optional_int(data, "agent_id"),
        )


@dataclass(frozen=True)
class OrderResult:
    order: Order
    decision: Decision | None = None
    fill: Fill | None = None
    portfolio: PortfolioSnapshot | None = None
    raw: dict[str, Any] = field(repr=False, default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "OrderResult":
        fill_data = data.get("fill")
        decision_data = data.get("decision")
        portfolio_data = data.get("portfolio")
        return cls(
            order=Order.from_dict(data.get("order", {})),
            decision=Decision.from_dict(decision_data) if isinstance(decision_data, dict) else None,
            fill=Fill.from_dict(fill_data) if isinstance(fill_data, dict) else None,
            portfolio=PortfolioSnapshot.from_dict(portfolio_data) if isinstance(portfolio_data, dict) else None,
            raw=data,
        )


@dataclass(frozen=True)
class LeaderboardRow:
    rank: int
    team_slug: str
    team_name: str
    composite_score: int
    equity_cents: int
    return_bps: int
    max_drawdown_bps: int
    brier_score_bps: int
    trade_count: int
    gross_exposure_cents: int
    last_heartbeat: str = ""
    status: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "LeaderboardRow":
        return cls(
            rank=_int(data, "rank"),
            team_slug=_string(data, "team_slug"),
            team_name=_string(data, "team_name"),
            composite_score=_int(data, "composite_score"),
            equity_cents=_int(data, "equity_cents"),
            return_bps=_int(data, "return_bps"),
            max_drawdown_bps=_int(data, "max_drawdown_bps"),
            brier_score_bps=_int(data, "brier_score_bps"),
            trade_count=_int(data, "trade_count"),
            gross_exposure_cents=_int(data, "gross_exposure_cents"),
            last_heartbeat=_string(data, "last_heartbeat"),
            status=_string(data, "status"),
        )


@dataclass(frozen=True)
class LeaderboardResponse:
    round: Round | None
    rows: list[LeaderboardRow]
    raw: dict[str, Any] = field(repr=False, default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "LeaderboardResponse":
        round_data = data.get("round")
        return cls(
            round=Round.from_dict(round_data) if isinstance(round_data, dict) else None,
            rows=[LeaderboardRow.from_dict(item) for item in data.get("rows", [])],
            raw=data,
        )
