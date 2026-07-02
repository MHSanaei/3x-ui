from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class PaidMessagePriceChanged(TelegramObject):
    """
    Describes a service message about a change in the price of paid messages within a chat.

    Source: https://core.telegram.org/bots/api#paidmessagepricechanged
    """

    paid_message_star_count: int
    """The new number of Telegram Stars that must be paid by non-administrator users of the supergroup chat for each sent message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, paid_message_star_count: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(paid_message_star_count=paid_message_star_count, **__pydantic_kwargs)
