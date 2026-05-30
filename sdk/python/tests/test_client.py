from __future__ import annotations

import json
import os
import unittest
from typing import Any
from unittest.mock import patch

from arena_client import (
    ArenaAPIError,
    ArenaClient,
    AuthenticationError,
    ConflictError,
    ForbiddenError,
    Fill,
    RateLimitError,
    RiskRejectedError,
)


class FakeTransport:
    def __init__(self, responses: dict[tuple[str, str], tuple[int, dict[str, Any]]]) -> None:
        self.responses = responses
        self.calls: list[dict[str, Any]] = []

    def __call__(
        self,
        method: str,
        url: str,
        body: bytes | None,
        headers: dict[str, str],
        timeout: float,
    ) -> tuple[int, bytes, dict[str, str]]:
        self.calls.append({"method": method, "url": url, "body": body, "headers": headers, "timeout": timeout})
        key = (method, url)
        if key not in self.responses:
            return 404, _json({"error": {"code": "not_found", "message": "not found"}}), {}
        status, payload = self.responses[key]
        return status, _json(payload), {"Content-Type": "application/json"}


class SequenceTransport:
    def __init__(self, responses: list[tuple[int, dict[str, Any], dict[str, str]] | ArenaAPIError]) -> None:
        self.responses = responses
        self.calls: list[dict[str, Any]] = []

    def __call__(
        self,
        method: str,
        url: str,
        body: bytes | None,
        headers: dict[str, str],
        timeout: float,
    ) -> tuple[int, bytes, dict[str, str]]:
        self.calls.append({"method": method, "url": url, "body": body, "headers": headers, "timeout": timeout})
        response = self.responses[min(len(self.calls) - 1, len(self.responses) - 1)]
        if isinstance(response, ArenaAPIError):
            raise response
        status, payload, response_headers = response
        return status, _json(payload), response_headers


class ArenaClientTests(unittest.TestCase):
    def test_from_env_reads_base_url_token_and_timeout(self) -> None:
        transport = FakeTransport({})
        with patch.dict(
            os.environ,
            {
                "ARENA_BASE_URL": "http://arena.local/",
                "ARENA_API_TOKEN": "paa_agent_test",
                "ARENA_TIMEOUT_SECONDS": "3.5",
                "ARENA_MAX_RETRIES": "2",
                "ARENA_RETRY_BACKOFF_SECONDS": "0.25",
            },
            clear=True,
        ):
            client = ArenaClient.from_env(transport=transport)

        self.assertEqual(client.base_url, "http://arena.local")
        self.assertEqual(client.api_token, "paa_agent_test")
        self.assertEqual(client.timeout, 3.5)
        self.assertEqual(client.max_retries, 2)
        self.assertEqual(client.retry_backoff_seconds, 0.25)

    def test_from_env_requires_token(self) -> None:
        with patch.dict(os.environ, {"ARENA_BASE_URL": "http://arena.local"}, clear=True):
            with self.assertRaisesRegex(ValueError, "ARENA_API_TOKEN"):
                ArenaClient.from_env()

    def test_me_parses_identity(self) -> None:
        transport = FakeTransport(
            {
                ("GET", "http://arena/api/v1/me"): (
                    200,
                    {
                        "team": {"id": 1, "slug": "team-01", "name": "Team 01", "is_active": True},
                        "agent": {"id": 7, "team_id": 1, "team_slug": "team-01", "slug": "default", "name": "Default", "status": "active", "kind": "agent"},
                        "active_round": {"id": 2, "slug": "practice-1", "name": "Practice 1", "mode": "practice", "status": "active", "initial_balance_cents": 1000000},
                        "legacy_team_auth": False,
                    },
                )
            }
        )

        result = ArenaClient("http://arena", "paa_agent_test", transport=transport).me()

        self.assertEqual(result.team.slug, "team-01")
        self.assertIsNotNone(result.agent)
        self.assertEqual(result.agent.id, 7)
        self.assertEqual(result.active_round.slug if result.active_round else "", "practice-1")
        self.assertFalse(result.legacy_team_auth)
        self.assertEqual(transport.calls[0]["headers"]["Authorization"], "Bearer paa_agent_test")

    def test_markets_portfolio_fills_and_leaderboard_parse(self) -> None:
        transport = FakeTransport(
            {
                ("GET", "http://arena/api/v1/markets"): (
                    200,
                    {"round": _round(), "markets": [_market()]},
                ),
                ("GET", "http://arena/api/v1/portfolio"): (
                    200,
                    {"round": _round(), "team": _team(), "portfolio": _portfolio()},
                ),
                ("GET", "http://arena/api/v1/fills"): (
                    200,
                    {"round": _round(), "fills": [_fill()]},
                ),
                ("GET", "http://arena/api/v1/leaderboard"): (
                    200,
                    {
                        "round": _round(),
                        "rows": [
                            {
                                "rank": 1,
                                "team_slug": "team-01",
                                "team_name": "Team 01",
                                "composite_score": 72,
                                "equity_cents": 1010000,
                                "return_bps": 100,
                                "max_drawdown_bps": 0,
                                "brier_score_bps": 2200,
                                "trade_count": 3,
                                "gross_exposure_cents": 50000,
                                "last_heartbeat": "2026-05-06T00:00:00Z",
                                "status": "online",
                            }
                        ],
                    },
                ),
            }
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport)

        self.assertEqual(client.markets().markets[0].slug, "market-1")
        self.assertEqual(client.portfolio().portfolio.equity_cents, 1005000)
        fill = client.fills()[0]
        self.assertEqual(fill.fill_price_bps, 5700)
        self.assertEqual(fill.round_id, 2)
        self.assertEqual(fill.team_id, 1)
        self.assertEqual(client.leaderboard().rows[0].team_slug, "team-01")

    def test_fill_preserves_absent_optional_ids(self) -> None:
        fill = Fill.from_dict(
            {
                "id": 5,
                "order_id": 9,
                "market_id": 1,
                "action": "buy",
                "outcome": "yes",
                "amount_cents": 10000,
                "fill_price_bps": 5700,
                "fee_cents": 0,
                "slippage_bps": 0,
            }
        )

        self.assertIsNone(fill.round_id)
        self.assertIsNone(fill.team_id)
        self.assertIsNone(fill.agent_id)

    def test_order_serializes_payload_and_parses_result(self) -> None:
        transport = FakeTransport(
            {
                ("POST", "http://arena/api/v1/orders"): (
                    201,
                    {
                        "decision": _decision(),
                        "order": _order("filled"),
                        "fill": _fill(),
                        "portfolio": _portfolio(),
                    },
                )
            }
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport)

        result = client.order(
            market_id=1,
            outcome="yes",
            action="buy",
            amount_cents=10000,
            limit_price_bps=5700,
            estimated_probability_bps=6400,
            confidence="medium",
            reason="Estimate is above market price.",
            client_order_id="order-1",
        )

        self.assertEqual(result.order.status, "filled")
        self.assertEqual(result.order.venue_order_id, "fake-9")
        self.assertEqual(result.order.client_order_id, "order-1")
        self.assertEqual(result.order.dispatched_at, "2026-05-06T00:00:01Z")
        self.assertIsNotNone(result.decision)
        sent = json.loads(transport.calls[0]["body"].decode("utf-8"))
        self.assertEqual(sent["market_id"], 1)
        self.assertEqual(sent["estimated_probability_bps"], 6400)
        self.assertEqual(sent["client_order_id"], "order-1")
        self.assertNotIn("prior_decision_id", sent)

    def test_order_generates_client_order_id_when_omitted(self) -> None:
        transport = FakeTransport(
            {
                ("POST", "http://arena/api/v1/orders"): (
                    201,
                    {
                        "decision": _decision(),
                        "order": _order("filled"),
                        "fill": _fill(),
                    },
                )
            }
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport)

        client.order(
            market_id=1,
            outcome="yes",
            action="buy",
            amount_cents=10000,
            limit_price_bps=5700,
            estimated_probability_bps=6400,
            confidence="medium",
            reason="Estimate is above market price.",
        )

        sent = json.loads(transport.calls[0]["body"].decode("utf-8"))
        self.assertRegex(sent["client_order_id"], r"^sdk-[0-9a-f-]+$")

    def test_decision_and_cancel_parse(self) -> None:
        transport = FakeTransport(
            {
                ("POST", "http://arena/api/v1/decisions"): (201, _decision()),
                ("POST", "http://arena/api/v1/orders/9/cancel"): (200, _order("canceled")),
            }
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport)

        decision = client.decision(
            market_id=1,
            outcome="yes",
            action="buy",
            amount_cents=10000,
            limit_price_bps=5700,
            estimated_probability_bps=6400,
            confidence="medium",
            reason="Estimate is above market price.",
            prior_decision_id=4,
        )
        canceled = client.cancel_order(9)

        self.assertEqual(decision.id, 11)
        self.assertEqual(canceled.status, "canceled")
        sent = json.loads(transport.calls[0]["body"].decode("utf-8"))
        self.assertEqual(sent["prior_decision_id"], 4)

    def test_heartbeat_posts_status_and_metadata(self) -> None:
        transport = FakeTransport({("POST", "http://arena/api/v1/heartbeat"): (201, {"status": "online"})})

        result = ArenaClient("http://arena", "paa_agent_test", transport=transport).heartbeat(metadata={"loop": 1})

        self.assertEqual(result["status"], "online")
        sent = json.loads(transport.calls[0]["body"].decode("utf-8"))
        self.assertEqual(sent["metadata"], {"loop": 1})

    def test_structured_errors(self) -> None:
        cases: list[tuple[int, str, type[ArenaAPIError]]] = [
            (401, "missing_token", AuthenticationError),
            (403, "inactive_agent", ForbiddenError),
            (409, "round_paused", ConflictError),
            (429, "rate_limit_exceeded", RateLimitError),
            (400, "amount_too_large", RiskRejectedError),
            (500, "internal_error", ArenaAPIError),
        ]
        for status, code, expected in cases:
            with self.subTest(code=code):
                transport = FakeTransport({("GET", "http://arena/api/v1/me"): (status, {"error": {"code": code, "message": "boom", "details": {"field": "x"}}})})
                client = ArenaClient("http://arena", "paa_agent_test", transport=transport)
                with self.assertRaises(expected) as caught:
                    client.me()
                self.assertEqual(caught.exception.status, status)
                self.assertEqual(caught.exception.code, code)
                self.assertEqual(caught.exception.details["field"], "x")

    def test_non_json_error_preserves_http_status(self) -> None:
        def transport(
            method: str,
            url: str,
            body: bytes | None,
            headers: dict[str, str],
            timeout: float,
        ) -> tuple[int, bytes, dict[str, str]]:
            return 502, b"<html>bad gateway</html>", {"Content-Type": "text/html"}

        client = ArenaClient("http://arena", "paa_agent_test", transport=transport)

        with self.assertRaises(ArenaAPIError) as caught:
            client.me()

        self.assertEqual(caught.exception.status, 502)
        self.assertEqual(caught.exception.code, "http_error")

    def test_retries_network_error_then_succeeds(self) -> None:
        sleeps: list[float] = []
        transport = SequenceTransport(
            [
                ArenaAPIError(0, "network_error", "connection reset", body={}),
                (200, {"team": _team(), "agent": None, "active_round": _round(), "legacy_team_auth": False}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, retry_backoff_seconds=0.5, sleep=sleeps.append)

        result = client.me()

        self.assertEqual(result.team.slug, "team-01")
        self.assertEqual(len(transport.calls), 2)
        self.assertEqual(sleeps, [0.5])

    def test_retries_transient_http_status_then_succeeds(self) -> None:
        sleeps: list[float] = []
        transport = SequenceTransport(
            [
                (503, {"error": {"code": "unavailable", "message": "try later"}}, {"Content-Type": "application/json"}),
                (200, {"team": _team(), "agent": None, "active_round": _round(), "legacy_team_auth": False}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, retry_backoff_seconds=0.25, sleep=sleeps.append)

        result = client.me()

        self.assertEqual(result.team.slug, "team-01")
        self.assertEqual(len(transport.calls), 2)
        self.assertEqual(sleeps, [0.25])

    def test_does_not_retry_risk_rejection_or_auth_errors(self) -> None:
        cases: list[tuple[int, str, type[ArenaAPIError]]] = [
            (400, "amount_too_large", RiskRejectedError),
            (401, "invalid_token", AuthenticationError),
            (403, "inactive_agent", ForbiddenError),
            (409, "round_paused", ConflictError),
        ]
        for status, code, expected in cases:
            with self.subTest(code=code):
                transport = FakeTransport({("GET", "http://arena/api/v1/me"): (status, {"error": {"code": code, "message": "stop"}})})
                client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, sleep=lambda _: None)
                with self.assertRaises(expected):
                    client.me()
                self.assertEqual(len(transport.calls), 1)

    def test_retries_429_only_with_retry_after_header(self) -> None:
        without_retry_after = SequenceTransport(
            [
                (429, {"error": {"code": "rate_limit_exceeded", "message": "slow down"}}, {"Content-Type": "application/json"}),
                (200, {"team": _team(), "agent": None, "active_round": _round(), "legacy_team_auth": False}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=without_retry_after, max_retries=2, sleep=lambda _: None)
        with self.assertRaises(RateLimitError):
            client.me()
        self.assertEqual(len(without_retry_after.calls), 1)

        sleeps: list[float] = []
        with_retry_after = SequenceTransport(
            [
                (429, {"error": {"code": "rate_limit_exceeded", "message": "slow down"}}, {"Content-Type": "application/json", "Retry-After": "0.2"}),
                (200, {"team": _team(), "agent": None, "active_round": _round(), "legacy_team_auth": False}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=with_retry_after, max_retries=2, sleep=sleeps.append)

        result = client.me()

        self.assertEqual(result.team.slug, "team-01")
        self.assertEqual(len(with_retry_after.calls), 2)
        self.assertEqual(sleeps, [0.2])

    def test_rejects_nan_retry_after_without_sleeping(self) -> None:
        sleeps: list[float] = []
        transport = SequenceTransport(
            [
                (429, {"error": {"code": "rate_limit_exceeded", "message": "slow down"}}, {"Content-Type": "application/json", "Retry-After": "NaN"}),
                (200, {"team": _team(), "agent": None, "active_round": _round(), "legacy_team_auth": False}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, sleep=sleeps.append)

        with self.assertRaises(RateLimitError):
            client.me()

        self.assertEqual(len(transport.calls), 1)
        self.assertEqual(sleeps, [])

    def test_does_not_retry_order_post(self) -> None:
        transport = SequenceTransport(
            [
                (503, {"error": {"code": "unavailable", "message": "try later"}}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, sleep=lambda _: None)

        with self.assertRaises(ArenaAPIError) as caught:
            client.order(
                market_id=1,
                outcome="yes",
                action="buy",
                amount_cents=10000,
                limit_price_bps=5700,
                estimated_probability_bps=6400,
                confidence="medium",
                reason="Estimate is above market price.",
            )

        self.assertEqual(caught.exception.status, 503)
        self.assertEqual(len(transport.calls), 1)

    def test_does_not_retry_decision_post(self) -> None:
        transport = SequenceTransport(
            [
                (503, {"error": {"code": "unavailable", "message": "try later"}}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, sleep=lambda _: None)

        with self.assertRaises(ArenaAPIError) as caught:
            client.decision(
                market_id=1,
                outcome="yes",
                action="buy",
                amount_cents=10000,
                limit_price_bps=5700,
                estimated_probability_bps=6400,
                confidence="medium",
                reason="Estimate is above market price.",
            )

        self.assertEqual(caught.exception.status, 503)
        self.assertEqual(len(transport.calls), 1)

    def test_does_not_retry_cancel_order_post(self) -> None:
        transport = SequenceTransport(
            [
                (503, {"error": {"code": "unavailable", "message": "try later"}}, {"Content-Type": "application/json"}),
            ]
        )
        client = ArenaClient("http://arena", "paa_agent_test", transport=transport, max_retries=2, sleep=lambda _: None)

        with self.assertRaises(ArenaAPIError) as caught:
            client.cancel_order(9)

        self.assertEqual(caught.exception.status, 503)
        self.assertEqual(len(transport.calls), 1)


def _json(payload: dict[str, Any]) -> bytes:
    return json.dumps(payload).encode("utf-8")


def _team() -> dict[str, Any]:
    return {"id": 1, "slug": "team-01", "name": "Team 01", "is_active": True}


def _round() -> dict[str, Any]:
    return {"id": 2, "slug": "practice-1", "name": "Practice 1", "mode": "practice", "status": "active", "initial_balance_cents": 1000000}


def _market() -> dict[str, Any]:
    return {"id": 1, "venue": "fake", "external_id": "fake-1", "slug": "market-1", "title": "Demo market", "category": "demo", "status": "open", "yes_price_bps": 5700, "no_price_bps": 4300}


def _portfolio() -> dict[str, Any]:
    return {"cash_cents": 995000, "equity_cents": 1005000, "realized_pnl_cents": 0, "unrealized_pnl_cents": 5000, "gross_exposure_cents": 10000, "max_drawdown_bps": 0, "created_at": "2026-05-06T00:00:00Z"}


def _decision() -> dict[str, Any]:
    return {"id": 11, "round_id": 2, "team_id": 1, "agent_id": 7, "market_id": 1, "observed_price_bps": 5700, "estimated_probability_bps": 6400, "edge_bps": 700, "action": "buy", "outcome": "yes", "amount_cents": 10000, "confidence": "medium", "reason": "Estimate is above market price.", "created_at": "2026-05-06T00:00:00Z"}


def _order(status: str) -> dict[str, Any]:
    return {"id": 9, "round_id": 2, "team_id": 1, "agent_id": 7, "market_id": 1, "venue_order_id": "fake-9", "client_order_id": "order-1", "action": "buy", "outcome": "yes", "amount_cents": 10000, "limit_price_bps": 5700, "status": status, "dispatched_at": "2026-05-06T00:00:01Z", "created_at": "2026-05-06T00:00:00Z"}


def _fill() -> dict[str, Any]:
    return {"id": 5, "round_id": 2, "team_id": 1, "agent_id": 7, "order_id": 9, "market_id": 1, "action": "buy", "outcome": "yes", "amount_cents": 10000, "fill_price_bps": 5700, "fee_cents": 0, "slippage_bps": 0, "created_at": "2026-05-06T00:00:00Z"}


if __name__ == "__main__":
    unittest.main()
