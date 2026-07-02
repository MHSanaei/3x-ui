from __future__ import annotations

from collections.abc import Generator
from typing import TYPE_CHECKING, Any, Final

from .event.bases import REJECTED, UNHANDLED
from .event.event import EventObserver
from .event.telegram import TelegramEventObserver

if TYPE_CHECKING:
    from aiogram.types import TelegramObject

INTERNAL_UPDATE_TYPES: Final[frozenset[str]] = frozenset({"update", "error"})


class Router:
    """
    Router can route update, and it nested update types like messages, callback query,
    polls and all other event types.

    Event handlers can be registered in observer by two ways:

    - By observer method - :obj:`router.<event_type>.register(handler, <filters, ...>)`
    - By decorator - :obj:`@router.<event_type>(<filters, ...>)`
    """

    def __init__(self, *, name: str | None = None) -> None:
        """
        :param name: Optional router name, can be useful for debugging
        """

        self.name = name or hex(id(self))

        self._parent_router: Router | None = None
        self.sub_routers: list[Router] = []

        # Observers
        self.message = TelegramEventObserver(router=self, event_name="message")
        self.edited_message = TelegramEventObserver(router=self, event_name="edited_message")
        self.channel_post = TelegramEventObserver(router=self, event_name="channel_post")
        self.edited_channel_post = TelegramEventObserver(
            router=self,
            event_name="edited_channel_post",
        )
        self.inline_query = TelegramEventObserver(router=self, event_name="inline_query")
        self.chosen_inline_result = TelegramEventObserver(
            router=self,
            event_name="chosen_inline_result",
        )
        self.callback_query = TelegramEventObserver(router=self, event_name="callback_query")
        self.shipping_query = TelegramEventObserver(router=self, event_name="shipping_query")
        self.pre_checkout_query = TelegramEventObserver(
            router=self,
            event_name="pre_checkout_query",
        )
        self.poll = TelegramEventObserver(router=self, event_name="poll")
        self.poll_answer = TelegramEventObserver(router=self, event_name="poll_answer")
        self.my_chat_member = TelegramEventObserver(router=self, event_name="my_chat_member")
        self.chat_member = TelegramEventObserver(router=self, event_name="chat_member")
        self.chat_join_request = TelegramEventObserver(router=self, event_name="chat_join_request")
        self.message_reaction = TelegramEventObserver(router=self, event_name="message_reaction")
        self.message_reaction_count = TelegramEventObserver(
            router=self,
            event_name="message_reaction_count",
        )
        self.chat_boost = TelegramEventObserver(router=self, event_name="chat_boost")
        self.removed_chat_boost = TelegramEventObserver(
            router=self,
            event_name="removed_chat_boost",
        )
        self.deleted_business_messages = TelegramEventObserver(
            router=self,
            event_name="deleted_business_messages",
        )
        self.business_connection = TelegramEventObserver(
            router=self,
            event_name="business_connection",
        )
        self.edited_business_message = TelegramEventObserver(
            router=self,
            event_name="edited_business_message",
        )
        self.business_message = TelegramEventObserver(router=self, event_name="business_message")
        self.purchased_paid_media = TelegramEventObserver(
            router=self,
            event_name="purchased_paid_media",
        )
        self.managed_bot = TelegramEventObserver(router=self, event_name="managed_bot")
        self.guest_message = TelegramEventObserver(router=self, event_name="guest_message")

        self.errors = self.error = TelegramEventObserver(router=self, event_name="error")

        self.startup = EventObserver()
        self.shutdown = EventObserver()

        self.observers: dict[str, TelegramEventObserver] = {
            "message": self.message,
            "edited_message": self.edited_message,
            "channel_post": self.channel_post,
            "edited_channel_post": self.edited_channel_post,
            "inline_query": self.inline_query,
            "chosen_inline_result": self.chosen_inline_result,
            "callback_query": self.callback_query,
            "shipping_query": self.shipping_query,
            "pre_checkout_query": self.pre_checkout_query,
            "poll": self.poll,
            "poll_answer": self.poll_answer,
            "my_chat_member": self.my_chat_member,
            "chat_member": self.chat_member,
            "chat_join_request": self.chat_join_request,
            "message_reaction": self.message_reaction,
            "message_reaction_count": self.message_reaction_count,
            "chat_boost": self.chat_boost,
            "removed_chat_boost": self.removed_chat_boost,
            "deleted_business_messages": self.deleted_business_messages,
            "business_connection": self.business_connection,
            "edited_business_message": self.edited_business_message,
            "business_message": self.business_message,
            "purchased_paid_media": self.purchased_paid_media,
            "managed_bot": self.managed_bot,
            "guest_message": self.guest_message,
            "error": self.errors,
        }

    def __str__(self) -> str:
        return f"{type(self).__name__} {self.name!r}"

    def __repr__(self) -> str:
        return f"<{self}>"

    def resolve_used_update_types(self, skip_events: set[str] | None = None) -> list[str]:
        """
        Resolve registered event names

        Is useful for getting updates only for registered event types.

        :param skip_events: skip specified event names
        :return: sorted list of registered names
        """
        handlers_in_use: set[str] = set()
        if skip_events is None:
            skip_events = set()
        skip_events = {*skip_events, *INTERNAL_UPDATE_TYPES}

        for router in self.chain_tail:
            for update_name, observer in router.observers.items():
                if observer.handlers and update_name not in skip_events:
                    handlers_in_use.add(update_name)

        return sorted(handlers_in_use)

    async def propagate_event(self, update_type: str, event: TelegramObject, **kwargs: Any) -> Any:
        kwargs.update(event_router=self)
        observer = self.observers.get(update_type)

        async def _wrapped(telegram_event: TelegramObject, **data: Any) -> Any:
            return await self._propagate_event(
                observer=observer,
                update_type=update_type,
                event=telegram_event,
                **data,
            )

        if observer:
            return await observer.wrap_outer_middleware(_wrapped, event=event, data=kwargs)
        return await _wrapped(event, **kwargs)

    async def _propagate_event(
        self,
        observer: TelegramEventObserver | None,
        update_type: str,
        event: TelegramObject,
        **kwargs: Any,
    ) -> Any:
        response = UNHANDLED
        if observer:
            # Check globally defined filters before any other handler will be checked.
            # This check is placed here instead of `trigger` method to add possibility
            # to pass context to handlers from global filters.
            result, data = await observer.check_root_filters(event, **kwargs)
            if not result:
                return UNHANDLED
            kwargs.update(data)

            response = await observer.trigger(event, **kwargs)
            if response is REJECTED:  # pragma: no cover
                # Possible only if some handler returns REJECTED
                return UNHANDLED
            if response is not UNHANDLED:
                return response

        for router in self.sub_routers:
            response = await router.propagate_event(update_type=update_type, event=event, **kwargs)
            if response is not UNHANDLED:
                break

        return response

    @property
    def chain_head(self) -> Generator[Router, None, None]:
        router: Router | None = self
        while router:
            yield router
            router = router.parent_router

    @property
    def chain_tail(self) -> Generator[Router, None, None]:
        yield self
        for router in self.sub_routers:
            yield from router.chain_tail

    @property
    def parent_router(self) -> Router | None:
        return self._parent_router

    @parent_router.setter
    def parent_router(self, router: Router) -> None:
        """
        Internal property setter of parent router fot this router.
        Do not use this method in own code.
        All routers should be included via `include_router` method.

        Self- and circular- referencing are not allowed here

        :param router:
        """
        if not isinstance(router, Router):
            msg = f"router should be instance of Router not {type(router).__name__!r}"
            raise ValueError(msg)
        if self._parent_router:
            msg = f"Router is already attached to {self._parent_router!r}"
            raise RuntimeError(msg)
        if self == router:
            msg = "Self-referencing routers is not allowed"
            raise RuntimeError(msg)

        parent: Router | None = router
        while parent is not None:
            if parent == self:
                msg = "Circular referencing of Router is not allowed"
                raise RuntimeError(msg)

            parent = parent.parent_router

        self._parent_router = router
        router.sub_routers.append(self)

    def include_routers(self, *routers: Router) -> None:
        """
        Attach multiple routers.

        :param routers:
        :return:
        """
        if not routers:
            msg = "At least one router must be provided"
            raise ValueError(msg)
        for router in routers:
            self.include_router(router)

    def include_router(self, router: Router) -> Router:
        """
        Attach another router.

        :param router:
        :return:
        """
        if not isinstance(router, Router):
            msg = f"router should be instance of Router not {type(router).__name__!r}"
            raise ValueError(msg)
        router.parent_router = self
        return router

    async def emit_startup(self, *args: Any, **kwargs: Any) -> None:
        """
        Recursively call startup callbacks

        :param args:
        :param kwargs:
        :return:
        """
        kwargs.update(router=self)
        await self.startup.trigger(*args, **kwargs)
        for router in self.sub_routers:
            await router.emit_startup(*args, **kwargs)

    async def emit_shutdown(self, *args: Any, **kwargs: Any) -> None:
        """
        Recursively call shutdown callbacks to graceful shutdown

        :param args:
        :param kwargs:
        :return:
        """
        kwargs.update(router=self)
        await self.shutdown.trigger(*args, **kwargs)
        for router in self.sub_routers:
            await router.emit_shutdown(*args, **kwargs)
