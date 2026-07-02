from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import DateTimeUnion
from .base import TelegramMethod


class SetUserEmojiStatus(TelegramMethod[bool]):
    """
    Changes the emoji status for a given user that previously allowed the bot to manage their emoji status via the Mini App method `requestEmojiStatusAccess <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setuseremojistatus
    """

    __returning__ = bool
    __api_method__ = "setUserEmojiStatus"

    user_id: int
    """Unique identifier of the target user"""
    emoji_status_custom_emoji_id: str | None = None
    """Custom emoji identifier of the emoji status to set. Pass an empty string to remove the status"""
    emoji_status_expiration_date: DateTimeUnion | None = None
    """Expiration date of the emoji status, if any"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            emoji_status_custom_emoji_id: str | None = None,
            emoji_status_expiration_date: DateTimeUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                emoji_status_custom_emoji_id=emoji_status_custom_emoji_id,
                emoji_status_expiration_date=emoji_status_expiration_date,
                **__pydantic_kwargs,
            )
