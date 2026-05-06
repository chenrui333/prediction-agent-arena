from __future__ import annotations

import json
import os
from typing import Any, Callable, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from .errors import (
    ArenaAPIError,
    AuthenticationError,
    ConflictError,
    ForbiddenError,
    RateLimitError,
    RiskRejectedError,
)
from .models import (
    Decision,
    Fill,
    LeaderboardResponse,
    MarketsResponse,
    MeResponse,
    Order,
    OrderResult,
    PortfolioResponse,
)

Transport = Callable[[str, str, Optional[bytes], dict[str, str], float], tuple[int, bytes, dict[str, str]]]

RISK_ERROR_CODES = {
    "amount_too_large",
    "insufficient_cash",
    "insufficient_position",
    "limit_price_required",
    "malformed_limit_price",
    "malformed_probability",
    "market_exposure_exceeded",
    "max_open_orders_exceeded",
    "missing_estimated_probability",
    "missing_reason",
    "rate_limit_exceeded",
    "risk_limit_exceeded",
    "total_exposure_exceeded",
}


class ArenaClient:
    """Thin student client for the prediction-agent-arena HTTP API."""

    def __init__(
        self,
        base_url: str,
        api_token: str,
        timeout: float = 10.0,
        transport: Transport | None = None,
    ) -> None:
        if not base_url:
            raise ValueError("base_url is required")
        if not api_token:
            raise ValueError("api_token is required")
        self.base_url = base_url.rstrip("/")
        self.api_token = api_token
        self.timeout = timeout
        self._transport = transport or _urllib_transport

    @classmethod
    def from_env(cls, timeout: float | None = None, transport: Transport | None = None) -> "ArenaClient":
        base_url = os.environ.get("ARENA_BASE_URL", "http://localhost:8080")
        api_token = os.environ.get("ARENA_API_TOKEN", "")
        if not api_token:
            raise ValueError("ARENA_API_TOKEN is required")
        configured_timeout = timeout
        if configured_timeout is None:
            configured_timeout = float(os.environ.get("ARENA_TIMEOUT_SECONDS", "10"))
        return cls(base_url=base_url, api_token=api_token, timeout=configured_timeout, transport=transport)

    def me(self) -> MeResponse:
        return MeResponse.from_dict(self._request("GET", "/api/v1/me"))

    def markets(self) -> MarketsResponse:
        return MarketsResponse.from_dict(self._request("GET", "/api/v1/markets"))

    def portfolio(self) -> PortfolioResponse:
        return PortfolioResponse.from_dict(self._request("GET", "/api/v1/portfolio"))

    def heartbeat(self, status: str = "online", metadata: dict[str, Any] | None = None) -> dict[str, Any]:
        return self._request("POST", "/api/v1/heartbeat", {"status": status, "metadata": metadata or {}})

    def decision(
        self,
        *,
        market_id: int,
        outcome: str,
        action: str,
        amount_cents: int,
        limit_price_bps: int,
        estimated_probability_bps: int,
        confidence: str,
        reason: str,
        prior_decision_id: int | None = None,
    ) -> Decision:
        payload = _trade_payload(
            market_id=market_id,
            outcome=outcome,
            action=action,
            amount_cents=amount_cents,
            limit_price_bps=limit_price_bps,
            estimated_probability_bps=estimated_probability_bps,
            confidence=confidence,
            reason=reason,
            prior_decision_id=prior_decision_id,
        )
        return Decision.from_dict(self._request("POST", "/api/v1/decisions", payload))

    def order(
        self,
        *,
        market_id: int,
        outcome: str,
        action: str,
        amount_cents: int,
        limit_price_bps: int,
        estimated_probability_bps: int,
        confidence: str,
        reason: str,
        prior_decision_id: int | None = None,
    ) -> OrderResult:
        payload = _trade_payload(
            market_id=market_id,
            outcome=outcome,
            action=action,
            amount_cents=amount_cents,
            limit_price_bps=limit_price_bps,
            estimated_probability_bps=estimated_probability_bps,
            confidence=confidence,
            reason=reason,
            prior_decision_id=prior_decision_id,
        )
        return OrderResult.from_dict(self._request("POST", "/api/v1/orders", payload))

    def cancel_order(self, order_id: int) -> Order:
        return Order.from_dict(self._request("POST", f"/api/v1/orders/{order_id}/cancel"))

    def fills(self) -> list[Fill]:
        data = self._request("GET", "/api/v1/fills")
        return [Fill.from_dict(item) for item in data.get("fills", [])]

    def leaderboard(self) -> LeaderboardResponse:
        return LeaderboardResponse.from_dict(self._request("GET", "/api/v1/leaderboard"))

    def _request(self, method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
        body: bytes | None = None
        headers = {
            "Accept": "application/json",
            "Authorization": f"Bearer {self.api_token}",
            "User-Agent": "prediction-agent-arena-python-sdk/0.1",
        }
        if payload is not None:
            body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
            headers["Content-Type"] = "application/json"

        url = f"{self.base_url}{path}"
        status, response_body, _ = self._transport(method, url, body, headers, self.timeout)
        if status >= 400:
            try:
                data = _decode_json(response_body)
            except ArenaAPIError as err:
                raise ArenaAPIError(status, "http_error", f"arena returned HTTP {status}", body={}) from err
            raise _error_from_response(status, data)
        data = _decode_json(response_body)
        return data


def _trade_payload(
    *,
    market_id: int,
    outcome: str,
    action: str,
    amount_cents: int,
    limit_price_bps: int,
    estimated_probability_bps: int,
    confidence: str,
    reason: str,
    prior_decision_id: int | None = None,
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "market_id": market_id,
        "outcome": outcome,
        "action": action,
        "amount_cents": amount_cents,
        "limit_price_bps": limit_price_bps,
        "estimated_probability_bps": estimated_probability_bps,
        "confidence": confidence,
        "reason": reason,
    }
    if prior_decision_id is not None:
        payload["prior_decision_id"] = prior_decision_id
    return payload


def _urllib_transport(
    method: str,
    url: str,
    body: bytes | None,
    headers: dict[str, str],
    timeout: float,
) -> tuple[int, bytes, dict[str, str]]:
    request = Request(url=url, data=body, headers=headers, method=method)
    try:
        with urlopen(request, timeout=timeout) as response:
            return response.status, response.read(), dict(response.headers.items())
    except HTTPError as err:
        return err.code, err.read(), dict(err.headers.items())
    except URLError as err:
        raise ArenaAPIError(0, "network_error", str(err.reason), body={}) from err
    except TimeoutError as err:
        raise ArenaAPIError(0, "request_timeout", "arena request timed out", body={}) from err


def _decode_json(body: bytes) -> dict[str, Any]:
    if not body:
        return {}
    try:
        value = json.loads(body.decode("utf-8"))
    except json.JSONDecodeError as err:
        raise ArenaAPIError(0, "invalid_response", "arena returned invalid JSON", body={}) from err
    if isinstance(value, dict):
        return value
    return {"data": value}


def _error_from_response(status: int, data: dict[str, Any]) -> ArenaAPIError:
    error = data.get("error")
    if isinstance(error, dict):
        code = str(error.get("code", "api_error"))
        message = str(error.get("message", "arena API error"))
        details = error.get("details") if isinstance(error.get("details"), dict) else {}
    else:
        code = str(data.get("code", "api_error"))
        message = str(data.get("message", "arena API error"))
        details = data.get("details") if isinstance(data.get("details"), dict) else {}

    if status == 401:
        return AuthenticationError(status, code, message, details, data)
    if status == 403:
        return ForbiddenError(status, code, message, details, data)
    if status == 409:
        return ConflictError(status, code, message, details, data)
    if status == 429:
        return RateLimitError(status, code, message, details, data)
    if status == 400 and (code in RISK_ERROR_CODES or "violation" in data):
        return RiskRejectedError(status, code, message, details, data)
    return ArenaAPIError(status, code, message, details, data)
