from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class ReadBusinessMessage(TelegramMethod[bool]):
    """
    Marks incoming message as read on behalf of a business account. Requires the *can_read_messages* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#readbusinessmessage
    """

    __returning__ = bool
    __api_method__ = "readBusinessMessage"

    business_connection_id: str
    """Unique identifier of the business connection on behalf of which to read the message"""
    chat_id: int
    """Unique identifier of the chat in which the message was received. The chat must have been active in the last 24 hours"""
    message_id: int
    """Unique identifier of the message to mark as read"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            chat_id: int,
            message_id: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                chat_id=chat_id,
                message_id=message_id,
                **__pydantic_kwargs,
            )
