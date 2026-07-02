from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatMemberStatus
from .chat_member import ChatMember

if TYPE_CHECKING:
    from .user import User


class ChatMemberLeft(ChatMember):
    """
    Represents a `chat member <https://core.telegram.org/bots/api#chatmember>`_ that isn't currently a member of the chat, but may join it themselves.

    Source: https://core.telegram.org/bots/api#chatmemberleft
    """

    status: Literal[ChatMemberStatus.LEFT] = ChatMemberStatus.LEFT
    """The member's status in the chat, always 'left'"""
    user: User
    """Information about the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            status: Literal[ChatMemberStatus.LEFT] = ChatMemberStatus.LEFT,
            user: User,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(status=status, user=user, **__pydantic_kwargs)
