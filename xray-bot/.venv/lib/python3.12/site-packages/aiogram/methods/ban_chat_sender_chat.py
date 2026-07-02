from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class BanChatSenderChat(TelegramMethod[bool]):
    """
    Use this method to ban a channel chat in a supergroup or a channel. Until the chat is `unbanned <https://core.telegram.org/bots/api#unbanchatsenderchat>`_, the owner of the banned chat won't be able to send messages on behalf of **any of their channels**. The bot must be an administrator in the supergroup or channel for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#banchatsenderchat
    """

    __returning__ = bool
    __api_method__ = "banChatSenderChat"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    sender_chat_id: int
    """Unique identifier of the target sender chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            sender_chat_id: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, sender_chat_id=sender_chat_id, **__pydantic_kwargs)
