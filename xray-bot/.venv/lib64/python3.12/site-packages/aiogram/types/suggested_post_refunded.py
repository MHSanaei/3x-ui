from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message


class SuggestedPostRefunded(TelegramObject):
    """
    Describes a service message about a payment refund for a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostrefunded
    """

    reason: str
    """Reason for the refund. Currently, one of 'post_deleted' if the post was deleted within 24 hours of being posted or removed from scheduled messages without being posted, or 'payment_refunded' if the payer refunded their payment"""
    suggested_post_message: Message | None = None
    """*Optional*. Message containing the suggested post. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            reason: str,
            suggested_post_message: Message | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                reason=reason, suggested_post_message=suggested_post_message, **__pydantic_kwargs
            )
