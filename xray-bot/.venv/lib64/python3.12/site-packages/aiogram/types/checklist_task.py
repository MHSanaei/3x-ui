from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .message_entity import MessageEntity
    from .user import User


class ChecklistTask(TelegramObject):
    """
    Describes a task in a checklist.

    Source: https://core.telegram.org/bots/api#checklisttask
    """

    id: int
    """Unique identifier of the task"""
    text: str
    """Text of the task"""
    text_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the task text"""
    completed_by_user: User | None = None
    """*Optional*. User that completed the task; omitted if the task wasn't completed by a user"""
    completed_by_chat: Chat | None = None
    """*Optional*. Chat that completed the task; omitted if the task wasn't completed by a chat"""
    completion_date: int | None = None
    """*Optional*. Point in time (Unix timestamp) when the task was completed; 0 if the task wasn't completed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: int,
            text: str,
            text_entities: list[MessageEntity] | None = None,
            completed_by_user: User | None = None,
            completed_by_chat: Chat | None = None,
            completion_date: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                text=text,
                text_entities=text_entities,
                completed_by_user=completed_by_user,
                completed_by_chat=completed_by_chat,
                completion_date=completion_date,
                **__pydantic_kwargs,
            )
