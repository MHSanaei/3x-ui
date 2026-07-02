from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .custom import DateTime
    from .reaction_count import ReactionCount


class MessageReactionCountUpdated(TelegramObject):
    """
    This object represents reaction changes on a message with anonymous reactions.

    Source: https://core.telegram.org/bots/api#messagereactioncountupdated
    """

    chat: Chat
    """The chat containing the message"""
    message_id: int
    """Unique message identifier inside the chat"""
    date: DateTime
    """Date of the change in Unix time"""
    reactions: list[ReactionCount]
    """List of reactions that are present on the message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat: Chat,
            message_id: int,
            date: DateTime,
            reactions: list[ReactionCount],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat=chat,
                message_id=message_id,
                date=date,
                reactions=reactions,
                **__pydantic_kwargs,
            )
