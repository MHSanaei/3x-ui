from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message
    from .suggested_post_price import SuggestedPostPrice


class SuggestedPostApprovalFailed(TelegramObject):
    """
    Describes a service message about the failed approval of a suggested post. Currently, only caused by insufficient user funds at the time of approval.

    Source: https://core.telegram.org/bots/api#suggestedpostapprovalfailed
    """

    price: SuggestedPostPrice
    """Expected price of the post"""
    suggested_post_message: Message | None = None
    """*Optional*. Message containing the suggested post whose approval has failed. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            price: SuggestedPostPrice,
            suggested_post_message: Message | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                price=price, suggested_post_message=suggested_post_message, **__pydantic_kwargs
            )
