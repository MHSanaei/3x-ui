from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class ConvertGiftToStars(TelegramMethod[bool]):
    """
    Converts a given regular gift to Telegram Stars. Requires the *can_convert_gifts_to_stars* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#convertgifttostars
    """

    __returning__ = bool
    __api_method__ = "convertGiftToStars"

    business_connection_id: str
    """Unique identifier of the business connection"""
    owned_gift_id: str
    """Unique identifier of the regular gift that should be converted to Telegram Stars"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            owned_gift_id: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                owned_gift_id=owned_gift_id,
                **__pydantic_kwargs,
            )
