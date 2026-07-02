from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class DirectMessagePriceChanged(TelegramObject):
    """
    Describes a service message about a change in the price of direct messages sent to a channel chat.

    Source: https://core.telegram.org/bots/api#directmessagepricechanged
    """

    are_direct_messages_enabled: bool
    """:code:`True`, if direct messages are enabled for the channel chat; false otherwise"""
    direct_message_star_count: int | None = None
    """*Optional*. The new number of Telegram Stars that must be paid by users for each direct message sent to the channel. Does not apply to users who have been exempted by administrators. Defaults to 0"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            are_direct_messages_enabled: bool,
            direct_message_star_count: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                are_direct_messages_enabled=are_direct_messages_enabled,
                direct_message_star_count=direct_message_star_count,
                **__pydantic_kwargs,
            )
