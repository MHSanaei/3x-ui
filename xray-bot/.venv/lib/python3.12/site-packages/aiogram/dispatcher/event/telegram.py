from __future__ import annotations

from collections.abc import Callable
from typing import TYPE_CHECKING, Any

from aiogram.dispatcher.middlewares.manager import MiddlewareManager
from aiogram.exceptions import UnsupportedKeywordArgument
from aiogram.filters.base import Filter

from .bases import UNHANDLED, MiddlewareType, SkipHandler
from .handler import CallbackType, FilterObject, HandlerObject

if TYPE_CHECKING:
    from aiogram.dispatcher.router import Router
    from aiogram.types import TelegramObject


class TelegramEventObserver:
    """
    Event observer for Telegram events

    Here you can register handler with filter.
    This observer will stop event propagation when first handler is pass.
    """

    def __init__(self, router: Router, event_name: str) -> None:
        self.router: Router = router
        self.event_name: str = event_name

        self.handlers: list[HandlerObject] = []

        self.middleware = MiddlewareManager()
        self.outer_middleware = MiddlewareManager()

        # Re-used filters check method from already implemented handler object
        # with dummy callback which never will be used
        self._handler = HandlerObject(callback=lambda: True, filters=[])

    def filter(self, *filters: CallbackType) -> None:
        """
        Register filter for all handlers of this event observer

        :param filters: positional filters
        """
        if self._handler.filters is None:
            self._handler.filters = []
        self._handler.filters.extend([FilterObject(filter_) for filter_ in filters])

    def _resolve_middlewares(self) -> list[MiddlewareType[TelegramObject]]:
        middlewares: list[MiddlewareType[TelegramObject]] = []
        for router in reversed(tuple(self.router.chain_head)):
            observer = router.observers.get(self.event_name)
            if observer:
                middlewares.extend(observer.middleware)

        return middlewares

    def register(
        self,
        callback: CallbackType,
        *filters: CallbackType,
        flags: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> CallbackType:
        """
        Register event handler
        """
        if kwargs:
            msg = (
                "Passing any additional keyword arguments to the registrar method "
                "is not supported.\n"
                "This error may be caused when you are trying to register filters like in 2.x "
                "version of this framework, if it's true just look at correspoding "
                "documentation pages.\n"
                f"Please remove the {set(kwargs.keys())} arguments from this call.\n"
            )
            raise UnsupportedKeywordArgument(msg)

        if flags is None:
            flags = {}

        for item in filters:
            if isinstance(item, Filter):
                item.update_handler_flags(flags=flags)

        self.handlers.append(
            HandlerObject(
                callback=callback,
                filters=[FilterObject(filter_) for filter_ in filters],
                flags=flags,
            ),
        )

        return callback

    def wrap_outer_middleware(
        self,
        callback: Any,
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        wrapped_outer = self.middleware.wrap_middlewares(
            self.outer_middleware,
            callback,
        )
        return wrapped_outer(event, data)

    def check_root_filters(self, event: TelegramObject, **kwargs: Any) -> Any:
        return self._handler.check(event, **kwargs)

    async def trigger(self, event: TelegramObject, **kwargs: Any) -> Any:
        """
        Propagate event to handlers and stops propagation on first match.
        Handler will be called when all its filters are pass.
        """
        for handler in self.handlers:
            kwargs["handler"] = handler
            result, data = await handler.check(event, **kwargs)
            if result:
                kwargs.update(data)
                try:
                    wrapped_inner = self.outer_middleware.wrap_middlewares(
                        self._resolve_middlewares(),
                        handler.call,
                    )
                    return await wrapped_inner(event, kwargs)
                except SkipHandler:
                    continue

        return UNHANDLED

    def __call__(
        self,
        *filters: CallbackType,
        flags: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> Callable[[CallbackType], CallbackType]:
        """
        Decorator for registering event handlers
        """

        def wrapper(callback: CallbackType) -> CallbackType:
            self.register(callback, *filters, flags=flags, **kwargs)
            return callback

        return wrapper
