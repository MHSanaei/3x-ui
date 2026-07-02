from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat


class BusinessMessagesDeleted(TelegramObject):
    """
    This object is received when messages are deleted from a connected business account.

    Source: https://core.telegram.org/bots/api#businessmessagesdeleted
    """

    business_connection_id: str
    """Unique identifier of the business connection"""
    chat: Chat
    """Information about a chat in the business account. The bot may not have access to the chat or the corresponding user"""
    message_ids: list[int]
    """The list of identifiers of deleted messages in the chat of the business account"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            chat: Chat,
            message_ids: list[int],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                chat=chat,
                message_ids=message_ids,
                **__pydantic_kwargs,
            )
