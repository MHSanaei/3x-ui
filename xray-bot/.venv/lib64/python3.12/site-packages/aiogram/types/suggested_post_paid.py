from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message
    from .star_amount import StarAmount


class SuggestedPostPaid(TelegramObject):
    """
    Describes a service message about a successful payment for a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostpaid
    """

    currency: str
    """Currency in which the payment was made. Currently, one of 'XTR' for Telegram Stars or 'TON' for toncoins"""
    suggested_post_message: Message | None = None
    """*Optional*. Message containing the suggested post. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""
    amount: int | None = None
    """*Optional*. The amount of the currency that was received by the channel in nanotoncoins; for payments in toncoins only"""
    star_amount: StarAmount | None = None
    """*Optional*. The amount of Telegram Stars that was received by the channel; for payments in Telegram Stars only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            currency: str,
            suggested_post_message: Message | None = None,
            amount: int | None = None,
            star_amount: StarAmount | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                currency=currency,
                suggested_post_message=suggested_post_message,
                amount=amount,
                star_amount=star_amount,
                **__pydantic_kwargs,
            )
