from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class UnpinChatMessage(TelegramMethod[bool]):
    """
    Use this method to remove a message from the list of pinned messages in a chat. In private chats and channel direct messages chats, all messages can be unpinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to unpin messages in groups and channels respectively. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#unpinchatmessage
    """

    __returning__ = bool
    __api_method__ = "unpinChatMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be unpinned"""
    message_id: int | None = None
    """Identifier of the message to unpin. Required if *business_connection_id* is specified. If not specified, the most recent pinned message (by sending date) will be unpinned"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            business_connection_id: str | None = None,
            message_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                business_connection_id=business_connection_id,
                message_id=message_id,
                **__pydantic_kwargs,
            )
