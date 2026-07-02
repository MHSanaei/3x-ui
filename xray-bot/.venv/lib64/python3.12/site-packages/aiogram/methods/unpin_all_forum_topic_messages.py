from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class UnpinAllForumTopicMessages(TelegramMethod[bool]):
    """
    Use this method to clear the list of pinned messages in a forum topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_pin_messages* administrator right in the supergroup. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#unpinallforumtopicmessages
    """

    __returning__ = bool
    __api_method__ = "unpinAllForumTopicMessages"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    message_thread_id: int
    """Unique identifier for the target message thread of the forum topic"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_thread_id: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, message_thread_id=message_thread_id, **__pydantic_kwargs
            )
