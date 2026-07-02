from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class PinChatMessage(TelegramMethod[bool]):
    """
    Use this method to add a message to the list of pinned messages in a chat. In private chats and channel direct messages chats, all non-service messages can be pinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to pin messages in groups and channels respectively. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#pinchatmessage
    """

    __returning__ = bool
    __api_method__ = "pinChatMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    message_id: int
    """Identifier of a message to pin"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be pinned"""
    disable_notification: bool | None = None
    """Pass :code:`True` if it is not necessary to send a notification to all chat members about the new pinned message. Notifications are always disabled in channels and private chats"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_id: int,
            business_connection_id: str | None = None,
            disable_notification: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                message_id=message_id,
                business_connection_id=business_connection_id,
                disable_notification=disable_notification,
                **__pydantic_kwargs,
            )
