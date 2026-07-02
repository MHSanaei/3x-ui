from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .chat import Chat
    from .message_entity import MessageEntity
    from .poll_media import PollMedia
    from .user import User


class PollOption(TelegramObject):
    """
    This object contains information about one answer option in a poll.

    Source: https://core.telegram.org/bots/api#polloption
    """

    persistent_id: str
    """Unique identifier of the option, persistent on option addition and deletion"""
    text: str
    """Option text, 1-100 characters"""
    voter_count: int
    """Number of users who voted for this option; may be 0 if unknown"""
    text_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the option *text*. Currently, only custom emoji entities are allowed in poll option texts"""
    media: PollMedia | None = None
    """*Optional*. Media added to the poll option"""
    added_by_user: User | None = None
    """*Optional*. User who added the option; omitted if the option wasn't added by a user after poll creation"""
    added_by_chat: Chat | None = None
    """*Optional*. Chat that added the option; omitted if the option wasn't added by a chat after poll creation"""
    addition_date: DateTime | None = None
    """*Optional*. Point in time (Unix timestamp) when the option was added; omitted if the option existed in the original poll"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            persistent_id: str,
            text: str,
            voter_count: int,
            text_entities: list[MessageEntity] | None = None,
            media: PollMedia | None = None,
            added_by_user: User | None = None,
            added_by_chat: Chat | None = None,
            addition_date: DateTime | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                persistent_id=persistent_id,
                text=text,
                voter_count=voter_count,
                text_entities=text_entities,
                media=media,
                added_by_user=added_by_user,
                added_by_chat=added_by_chat,
                addition_date=addition_date,
                **__pydantic_kwargs,
            )
