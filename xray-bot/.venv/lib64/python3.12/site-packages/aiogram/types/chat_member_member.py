from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatMemberStatus
from .chat_member import ChatMember
from .custom import DateTime

if TYPE_CHECKING:
    from .user import User


class ChatMemberMember(ChatMember):
    """
    Represents a `chat member <https://core.telegram.org/bots/api#chatmember>`_ that has no additional privileges or restrictions.

    Source: https://core.telegram.org/bots/api#chatmembermember
    """

    status: Literal[ChatMemberStatus.MEMBER] = ChatMemberStatus.MEMBER
    """The member's status in the chat, always 'member'"""
    user: User
    """Information about the user"""
    tag: str | None = None
    """*Optional*. Tag of the member"""
    until_date: DateTime | None = None
    """*Optional*. Date when the user's subscription will expire; Unix time"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            status: Literal[ChatMemberStatus.MEMBER] = ChatMemberStatus.MEMBER,
            user: User,
            tag: str | None = None,
            until_date: DateTime | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                status=status, user=user, tag=tag, until_date=until_date, **__pydantic_kwargs
            )
