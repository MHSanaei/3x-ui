from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ResultChatMemberUnion
from .base import TelegramMethod


class GetChatAdministrators(TelegramMethod[list[ResultChatMemberUnion]]):
    """
    Use this method to get a list of administrators in a chat. Returns an Array of :class:`aiogram.types.chat_member.ChatMember` objects.

    Source: https://core.telegram.org/bots/api#getchatadministrators
    """

    __returning__ = list[ResultChatMemberUnion]
    __api_method__ = "getChatAdministrators"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`"""
    return_bots: bool | None = None
    """Pass :code:`True` to additionally receive all bots that are administrators of the chat. By default, bots other than the current bot are omitted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            return_bots: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, return_bots=return_bots, **__pydantic_kwargs)
