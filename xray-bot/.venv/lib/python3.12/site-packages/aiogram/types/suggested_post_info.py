from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .suggested_post_price import SuggestedPostPrice


class SuggestedPostInfo(TelegramObject):
    """
    Contains information about a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostinfo
    """

    state: str
    """State of the suggested post. Currently, it can be one of 'pending', 'approved', 'declined'"""
    price: SuggestedPostPrice | None = None
    """*Optional*. Proposed price of the post. If the field is omitted, then the post is unpaid"""
    send_date: DateTime | None = None
    """*Optional*. Proposed send date of the post. If the field is omitted, then the post can be published at any time within 30 days at the sole discretion of the user or administrator who approves it"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            state: str,
            price: SuggestedPostPrice | None = None,
            send_date: DateTime | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(state=state, price=price, send_date=send_date, **__pydantic_kwargs)
