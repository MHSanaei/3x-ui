from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat_boost import ChatBoost


class UserChatBoosts(TelegramObject):
    """
    This object represents a list of boosts added to a chat by a user.

    Source: https://core.telegram.org/bots/api#userchatboosts
    """

    boosts: list[ChatBoost]
    """The list of boosts added to the chat by the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, boosts: list[ChatBoost], **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(boosts=boosts, **__pydantic_kwargs)
