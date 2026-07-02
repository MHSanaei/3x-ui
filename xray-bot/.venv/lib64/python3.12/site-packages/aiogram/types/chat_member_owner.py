from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatMemberStatus
from .chat_member import ChatMember

if TYPE_CHECKING:
    from .user import User


class ChatMemberOwner(ChatMember):
    """
    Represents a `chat member <https://core.telegram.org/bots/api#chatmember>`_ that owns the chat and has all administrator privileges.

    Source: https://core.telegram.org/bots/api#chatmemberowner
    """

    status: Literal[ChatMemberStatus.CREATOR] = ChatMemberStatus.CREATOR
    """The member's status in the chat, always 'creator'"""
    user: User
    """Information about the user"""
    is_anonymous: bool
    """:code:`True`, if the user's presence in the chat is hidden"""
    custom_title: str | None = None
    """*Optional*. Custom title for this user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            status: Literal[ChatMemberStatus.CREATOR] = ChatMemberStatus.CREATOR,
            user: User,
            is_anonymous: bool,
            custom_title: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                status=status,
                user=user,
                is_anonymous=is_anonymous,
                custom_title=custom_title,
                **__pydantic_kwargs,
            )
