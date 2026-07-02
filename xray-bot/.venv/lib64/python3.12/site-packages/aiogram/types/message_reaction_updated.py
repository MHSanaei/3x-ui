from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .custom import DateTime
    from .reaction_type_union import ReactionTypeUnion
    from .user import User


class MessageReactionUpdated(TelegramObject):
    """
    This object represents a change of a reaction on a message performed by a user.

    Source: https://core.telegram.org/bots/api#messagereactionupdated
    """

    chat: Chat
    """The chat containing the message the user reacted to"""
    message_id: int
    """Unique identifier of the message inside the chat"""
    date: DateTime
    """Date of the change in Unix time"""
    old_reaction: list[ReactionTypeUnion]
    """Previous list of reaction types that were set by the user"""
    new_reaction: list[ReactionTypeUnion]
    """New list of reaction types that have been set by the user"""
    user: User | None = None
    """*Optional*. The user that changed the reaction, if the user isn't anonymous"""
    actor_chat: Chat | None = None
    """*Optional*. The chat on behalf of which the reaction was changed, if the user is anonymous"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat: Chat,
            message_id: int,
            date: DateTime,
            old_reaction: list[ReactionTypeUnion],
            new_reaction: list[ReactionTypeUnion],
            user: User | None = None,
            actor_chat: Chat | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat=chat,
                message_id=message_id,
                date=date,
                old_reaction=old_reaction,
                new_reaction=new_reaction,
                user=user,
                actor_chat=actor_chat,
                **__pydantic_kwargs,
            )
