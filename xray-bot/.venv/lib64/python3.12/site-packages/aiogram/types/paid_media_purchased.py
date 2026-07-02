from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class PaidMediaPurchased(TelegramObject):
    """
    This object contains information about a paid media purchase.

    Source: https://core.telegram.org/bots/api#paidmediapurchased
    """

    from_user: User = Field(..., alias="from")
    """User who purchased the media"""
    paid_media_payload: str
    """Bot-specified paid media payload"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            from_user: User,
            paid_media_payload: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                from_user=from_user, paid_media_payload=paid_media_payload, **__pydantic_kwargs
            )
