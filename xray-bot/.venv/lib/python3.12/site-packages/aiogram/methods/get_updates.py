from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import Update
from .base import TelegramMethod


class GetUpdates(TelegramMethod[list[Update]]):
    """
    Use this method to receive incoming updates using long polling (`wiki <https://en.wikipedia.org/wiki/Push_technology#Long_polling>`_). Returns an Array of :class:`aiogram.types.update.Update` objects.

     **Notes**

     **1.** This method will not work if an outgoing webhook is set up.

     **2.** In order to avoid getting duplicate updates, recalculate *offset* after each server response.

    Source: https://core.telegram.org/bots/api#getupdates
    """

    __returning__ = list[Update]
    __api_method__ = "getUpdates"

    offset: int | None = None
    """Identifier of the first update to be returned. Must be greater by one than the highest among the identifiers of previously received updates. By default, updates starting with the earliest unconfirmed update are returned. An update is considered confirmed as soon as :class:`aiogram.methods.get_updates.GetUpdates` is called with an *offset* higher than its *update_id*. The negative offset can be specified to retrieve updates starting from *-offset* update from the end of the updates queue. All previous updates will be forgotten"""
    limit: int | None = None
    """Limits the number of updates to be retrieved. Values between 1-100 are accepted. Defaults to 100"""
    timeout: int | None = None
    """Timeout in seconds for long polling. Defaults to 0, i.e. usual short polling. Should be positive, short polling should be used for testing purposes only"""
    allowed_updates: list[str] | None = None
    """A JSON-serialized list of the update types you want your bot to receive. For example, specify :code:`["message", "edited_channel_post", "callback_query"]` to only receive updates of these types. See :class:`aiogram.types.update.Update` for a complete list of available update types. Specify an empty list to receive all update types except *chat_member*, *message_reaction*, and *message_reaction_count* (default). If not specified, the previous setting will be used"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            offset: int | None = None,
            limit: int | None = None,
            timeout: int | None = None,
            allowed_updates: list[str] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                offset=offset,
                limit=limit,
                timeout=timeout,
                allowed_updates=allowed_updates,
                **__pydantic_kwargs,
            )
