from __future__ import annotations

from typing import Any


class ArenaAPIError(Exception):
    def __init__(
        self,
        status: int,
        code: str,
        message: str,
        details: dict[str, Any] | None = None,
        body: dict[str, Any] | None = None,
    ) -> None:
        super().__init__(f"{code}: {message}" if code else message)
        self.status = status
        self.code = code
        self.message = message
        self.details = details or {}
        self.body = body or {}


class AuthenticationError(ArenaAPIError):
    pass


class ForbiddenError(ArenaAPIError):
    pass


class ConflictError(ArenaAPIError):
    pass


class RateLimitError(ArenaAPIError):
    pass


class RiskRejectedError(ArenaAPIError):
    pass
