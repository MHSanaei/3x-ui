from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class TransferBusinessAccountStars(TelegramMethod[bool]):
    """
    Transfers Telegram Stars from the business account balance to the bot's balance. Requires the *can_transfer_stars* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#transferbusinessaccountstars
    """

    __returning__ = bool
    __api_method__ = "transferBusinessAccountStars"

    business_connection_id: str
    """Unique identifier of the business connection"""
    star_count: int
    """Number of Telegram Stars to transfer; 1-10000"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            star_count: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                star_count=star_count,
                **__pydantic_kwargs,
            )
