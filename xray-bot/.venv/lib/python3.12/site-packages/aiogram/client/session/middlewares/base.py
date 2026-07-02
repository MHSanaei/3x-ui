from __future__ import annotations

from abc import ABC, abstractmethod
from typing import TYPE_CHECKING, Protocol

from aiogram.methods.base import TelegramType

if TYPE_CHECKING:
    from aiogram.client.bot import Bot
    from aiogram.methods import Response, TelegramMethod


class NextRequestMiddlewareType(Protocol[TelegramType]):  # pragma: no cover
    async def __call__(
        self,
        bot: Bot,
        method: TelegramMethod[TelegramType],
    ) -> Response[TelegramType]:
        pass


class RequestMiddlewareType(Protocol):  # pragma: no cover
    async def __call__(
        self,
        make_request: NextRequestMiddlewareType[TelegramType],
        bot: Bot,
        method: TelegramMethod[TelegramType],
    ) -> Response[TelegramType]:
        pass


class BaseRequestMiddleware(ABC):
    """
    Generic middleware class
    """

    @abstractmethod
    async def __call__(
        self,
        make_request: NextRequestMiddlewareType[TelegramType],
        bot: Bot,
        method: TelegramMethod[TelegramType],
    ) -> Response[TelegramType]:
        """
        Execute middleware

        :param make_request: Wrapped make_request in middlewares chain
        :param bot: bot for request making
        :param method: Request method (Subclass of :class:`aiogram.methods.base.TelegramMethod`)

        :return: :class:`aiogram.methods.Response`
        """
