from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .chat_boost import ChatBoost


class ChatBoostUpdated(TelegramObject):
    """
    This object represents a boost added to a chat or changed.

    Source: https://core.telegram.org/bots/api#chatboostupdated
    """

    chat: Chat
    """Chat which was boosted"""
    boost: ChatBoost
    """Information about the chat boost"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat: Chat, boost: ChatBoost, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat=chat, boost=boost, **__pydantic_kwargs)
