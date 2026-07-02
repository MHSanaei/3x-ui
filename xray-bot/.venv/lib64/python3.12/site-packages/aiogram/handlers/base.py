from __future__ import annotations

from abc import ABC, abstractmethod
from typing import TYPE_CHECKING, Any, Generic, TypeVar, cast

from aiogram.types import Update

if TYPE_CHECKING:
    from aiogram import Bot

T = TypeVar("T")


class BaseHandlerMixin(Generic[T]):
    if TYPE_CHECKING:
        event: T
        data: dict[str, Any]


class BaseHandler(BaseHandlerMixin[T], ABC):
    """
    Base class for all class-based handlers
    """

    def __init__(self, event: T, **kwargs: Any) -> None:
        self.event: T = event
        self.data: dict[str, Any] = kwargs

    @property
    def bot(self) -> Bot:
        from aiogram import Bot

        if "bot" in self.data:
            return cast(Bot, self.data["bot"])
        msg = "Bot instance not found in the context"
        raise RuntimeError(msg)

    @property
    def update(self) -> Update:
        return cast(Update, self.data.get("update", self.data.get("event_update")))

    @abstractmethod
    async def handle(self) -> Any:  # pragma: no cover
        pass

    def __await__(self) -> Any:
        return self.handle().__await__()
