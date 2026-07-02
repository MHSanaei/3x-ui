from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .chat import Chat
    from .chat_boost_source_union import ChatBoostSourceUnion


class ChatBoostRemoved(TelegramObject):
    """
    This object represents a boost removed from a chat.

    Source: https://core.telegram.org/bots/api#chatboostremoved
    """

    chat: Chat
    """Chat which was boosted"""
    boost_id: str
    """Unique identifier of the boost"""
    remove_date: DateTime
    """Point in time (Unix timestamp) when the boost was removed"""
    source: ChatBoostSourceUnion
    """Source of the removed boost"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat: Chat,
            boost_id: str,
            remove_date: DateTime,
            source: ChatBoostSourceUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat=chat,
                boost_id=boost_id,
                remove_date=remove_date,
                source=source,
                **__pydantic_kwargs,
            )
