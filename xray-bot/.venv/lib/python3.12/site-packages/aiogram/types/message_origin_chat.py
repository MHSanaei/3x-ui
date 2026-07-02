from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import MessageOriginType
from .custom import DateTime
from .message_origin import MessageOrigin

if TYPE_CHECKING:
    from .chat import Chat


class MessageOriginChat(MessageOrigin):
    """
    The message was originally sent on behalf of a chat to a group chat.

    Source: https://core.telegram.org/bots/api#messageoriginchat
    """

    type: Literal[MessageOriginType.CHAT] = MessageOriginType.CHAT
    """Type of the message origin, always 'chat'"""
    date: DateTime
    """Date the message was sent originally in Unix time"""
    sender_chat: Chat
    """Chat that sent the message originally"""
    author_signature: str | None = None
    """*Optional*. For messages originally sent by an anonymous chat administrator, original message author signature"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MessageOriginType.CHAT] = MessageOriginType.CHAT,
            date: DateTime,
            sender_chat: Chat,
            author_signature: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                date=date,
                sender_chat=sender_chat,
                author_signature=author_signature,
                **__pydantic_kwargs,
            )
