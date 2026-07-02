from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatMemberStatus
from .chat_member import ChatMember
from .custom import DateTime

if TYPE_CHECKING:
    from .user import User


class ChatMemberBanned(ChatMember):
    """
    Represents a `chat member <https://core.telegram.org/bots/api#chatmember>`_ that was banned in the chat and can't return to the chat or view chat messages.

    Source: https://core.telegram.org/bots/api#chatmemberbanned
    """

    status: Literal[ChatMemberStatus.KICKED] = ChatMemberStatus.KICKED
    """The member's status in the chat, always 'kicked'"""
    user: User
    """Information about the user"""
    until_date: DateTime
    """Date when restrictions will be lifted for this user; Unix time. If 0, then the user is banned forever"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            status: Literal[ChatMemberStatus.KICKED] = ChatMemberStatus.KICKED,
            user: User,
            until_date: DateTime,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(status=status, user=user, until_date=until_date, **__pydantic_kwargs)
