from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ResultChatMemberUnion
from .base import TelegramMethod


class GetChatMember(TelegramMethod[ResultChatMemberUnion]):
    """
    Use this method to get information about a member of a chat. The method is only guaranteed to work for other users if the bot is an administrator in the chat. Returns a :class:`aiogram.types.chat_member.ChatMember` object on success.

    Source: https://core.telegram.org/bots/api#getchatmember
    """

    __returning__ = ResultChatMemberUnion
    __api_method__ = "getChatMember"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`"""
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
