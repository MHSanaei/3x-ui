import logging
from typing import TYPE_CHECKING, Any

from aiogram import loggers
from aiogram.methods import TelegramMethod
from aiogram.methods.base import Response, TelegramType

from .base import BaseRequestMiddleware, NextRequestMiddlewareType

if TYPE_CHECKING:
    from aiogram.client.bot import Bot

logger = logging.getLogger(__name__)


class RequestLogging(BaseRequestMiddleware):
    def __init__(self, ignore_methods: list[type[TelegramMethod[Any]]] | None = None):
        """
        Middleware for logging outgoing requests

        :param ignore_methods: methods to ignore in logging middleware
        """
        self.ignore_methods = ignore_methods or []

    async def __call__(
        self,
        make_request: NextRequestMiddlewareType[TelegramType],
        bot: "Bot",
        method: TelegramMethod[TelegramType],
    ) -> Response[TelegramType]:
        if type(method) not in self.ignore_methods:
            loggers.middlewares.info(
                "Make request with method=%r by bot id=%d",
                type(method).__name__,
                bot.id,
            )
        return await make_request(bot, method)
