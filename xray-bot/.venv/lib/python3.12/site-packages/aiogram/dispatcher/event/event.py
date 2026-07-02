from __future__ import annotations

from collections.abc import Callable
from typing import Any

from .handler import CallbackType, HandlerObject


class EventObserver:
    """
    Simple events observer

    Is used for managing events is not related with Telegram
    (For example startup/shutdown processes)

    Handlers can be registered via decorator or method

    .. code-block:: python

        <observer>.register(my_handler)

    .. code-block:: python

        @<observer>()
        async def my_handler(*args, **kwargs): ...
    """

    def __init__(self) -> None:
        self.handlers: list[HandlerObject] = []

    def register(self, callback: CallbackType) -> None:
        """
        Register callback with filters
        """
        self.handlers.append(HandlerObject(callback=callback))

    async def trigger(self, *args: Any, **kwargs: Any) -> None:
        """
        Propagate event to handlers.
        Handler will be called when all its filters is pass.
        """
        for handler in self.handlers:
            await handler.call(*args, **kwargs)

    def __call__(self) -> Callable[[CallbackType], CallbackType]:
        """
        Decorator for registering event handlers
        """

        def wrapper(callback: CallbackType) -> CallbackType:
            self.register(callback)
            return callback

        return wrapper
