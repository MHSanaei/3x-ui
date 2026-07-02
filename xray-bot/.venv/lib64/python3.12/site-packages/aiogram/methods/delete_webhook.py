from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class DeleteWebhook(TelegramMethod[bool]):
    """
    Use this method to remove webhook integration if you decide to switch back to :class:`aiogram.methods.get_updates.GetUpdates`. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletewebhook
    """

    __returning__ = bool
    __api_method__ = "deleteWebhook"

    drop_pending_updates: bool | None = None
    """Pass :code:`True` to drop all pending updates"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            drop_pending_updates: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(drop_pending_updates=drop_pending_updates, **__pydantic_kwargs)
