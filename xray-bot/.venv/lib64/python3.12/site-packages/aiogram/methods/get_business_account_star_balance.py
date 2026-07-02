from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import StarAmount
from .base import TelegramMethod


class GetBusinessAccountStarBalance(TelegramMethod[StarAmount]):
    """
    Returns the amount of Telegram Stars owned by a managed business account. Requires the *can_view_gifts_and_stars* business bot right. Returns :class:`aiogram.types.star_amount.StarAmount` on success.

    Source: https://core.telegram.org/bots/api#getbusinessaccountstarbalance
    """

    __returning__ = StarAmount
    __api_method__ = "getBusinessAccountStarBalance"

    business_connection_id: str
    """Unique identifier of the business connection"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, business_connection_id: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(business_connection_id=business_connection_id, **__pydantic_kwargs)
