from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any, NoReturn, TypeVar
from unittest.mock import sentinel

from aiogram.dispatcher.middlewares.base import BaseMiddleware
from aiogram.types import TelegramObject

MiddlewareEventType = TypeVar("MiddlewareEventType", bound=TelegramObject)
NextMiddlewareType = Callable[[MiddlewareEventType, dict[str, Any]], Awaitable[Any]]
MiddlewareType = (
    BaseMiddleware
    | Callable[
        [NextMiddlewareType[MiddlewareEventType], MiddlewareEventType, dict[str, Any]],
        Awaitable[Any],
    ]
)


UNHANDLED = sentinel.UNHANDLED
REJECTED = sentinel.REJECTED


class SkipHandler(Exception):
    pass


class CancelHandler(Exception):
    pass


def skip(message: str | None = None) -> NoReturn:
    """
    Raise an SkipHandler
    """
    raise SkipHandler(message or "Event skipped")
