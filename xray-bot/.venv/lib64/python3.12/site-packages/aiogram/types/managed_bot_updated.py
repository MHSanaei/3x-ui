from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class ManagedBotUpdated(TelegramObject):
    """
    This object contains information about the creation, token update, or owner update of a bot that is managed by the current bot.

    Source: https://core.telegram.org/bots/api#managedbotupdated
    """

    user: User
    """User that created the bot"""
    bot_user: User = Field(..., alias="bot")
    """Information about the bot. Token of the bot can be fetched using the method :class:`aiogram.methods.get_managed_bot_token.GetManagedBotToken`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, user: User, bot_user: User, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user=user, bot_user=bot_user, **__pydantic_kwargs)
