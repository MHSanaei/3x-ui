from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import MessageOriginType
from .custom import DateTime
from .message_origin import MessageOrigin

if TYPE_CHECKING:
    from .chat import Chat


class MessageOriginChannel(MessageOrigin):
    """
    The message was originally sent to a channel chat.

    Source: https://core.telegram.org/bots/api#messageoriginchannel
    """

    type: Literal[MessageOriginType.CHANNEL] = MessageOriginType.CHANNEL
    """Type of the message origin, always 'channel'"""
    date: DateTime
    """Date the message was sent originally in Unix time"""
    chat: Chat
    """Channel chat to which the message was originally sent"""
    message_id: int
    """Unique message identifier inside the chat"""
    author_signature: str | None = None
    """*Optional*. Signature of the original post author"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MessageOriginType.CHANNEL] = MessageOriginType.CHANNEL,
            date: DateTime,
            chat: Chat,
            message_id: int,
            author_signature: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                date=date,
                chat=chat,
                message_id=message_id,
                author_signature=author_signature,
                **__pydantic_kwargs,
            )
