from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import MessageOriginType
from .custom import DateTime
from .message_origin import MessageOrigin

if TYPE_CHECKING:
    from .user import User


class MessageOriginUser(MessageOrigin):
    """
    The message was originally sent by a known user.

    Source: https://core.telegram.org/bots/api#messageoriginuser
    """

    type: Literal[MessageOriginType.USER] = MessageOriginType.USER
    """Type of the message origin, always 'user'"""
    date: DateTime
    """Date the message was sent originally in Unix time"""
    sender_user: User
    """User that sent the message originally"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MessageOriginType.USER] = MessageOriginType.USER,
            date: DateTime,
            sender_user: User,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, date=date, sender_user=sender_user, **__pydantic_kwargs)
