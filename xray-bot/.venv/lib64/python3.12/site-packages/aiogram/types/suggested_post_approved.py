from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .message import Message
    from .suggested_post_price import SuggestedPostPrice


class SuggestedPostApproved(TelegramObject):
    """
    Describes a service message about the approval of a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostapproved
    """

    send_date: DateTime
    """Date when the post will be published"""
    suggested_post_message: Message | None = None
    """*Optional*. Message containing the suggested post. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""
    price: SuggestedPostPrice | None = None
    """*Optional*. Amount paid for the post"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            send_date: DateTime,
            suggested_post_message: Message | None = None,
            price: SuggestedPostPrice | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                send_date=send_date,
                suggested_post_message=suggested_post_message,
                price=price,
                **__pydantic_kwargs,
            )
