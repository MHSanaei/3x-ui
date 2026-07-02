from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class BotAccessSettings(TelegramObject):
    """
    This object describes the access settings of a bot.

    Source: https://core.telegram.org/bots/api#botaccesssettings
    """

    is_access_restricted: bool
    """:code:`True`, if only selected users can access the bot. The bot's owner can always access it"""
    added_users: list[User] | None = None
    """*Optional*. The list of other users who have access to the bot if the access is restricted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            is_access_restricted: bool,
            added_users: list[User] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                is_access_restricted=is_access_restricted,
                added_users=added_users,
                **__pydantic_kwargs,
            )
