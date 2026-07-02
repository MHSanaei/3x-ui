from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, UserChatBoosts
from .base import TelegramMethod


class GetUserChatBoosts(TelegramMethod[UserChatBoosts]):
    """
    Use this method to get the list of boosts added to a chat by a user. Requires administrator rights in the chat. Returns a :class:`aiogram.types.user_chat_boosts.UserChatBoosts` object.

    Source: https://core.telegram.org/bots/api#getuserchatboosts
    """

    __returning__ = UserChatBoosts
    __api_method__ = "getUserChatBoosts"

    chat_id: ChatIdUnion
    """Unique identifier for the chat or username of the channel in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat_id: ChatIdUnion, user_id: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, user_id=user_id, **__pydantic_kwargs)
