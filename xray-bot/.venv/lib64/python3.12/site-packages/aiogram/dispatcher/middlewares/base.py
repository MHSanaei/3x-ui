from abc import ABC, abstractmethod
from collections.abc import Awaitable, Callable
from typing import Any, TypeVar

from aiogram.types import TelegramObject

T = TypeVar("T")


class BaseMiddleware(ABC):
    """
    Generic middleware class
    """

    @abstractmethod
    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:  # pragma: no cover
        """
        Execute middleware

        :param handler: Wrapped handler in middlewares chain
        :param event: Incoming event (Subclass of :class:`aiogram.types.base.TelegramObject`)
        :param data: Contextual data. Will be mapped to handler arguments
        :return: :class:`Any`
        """
