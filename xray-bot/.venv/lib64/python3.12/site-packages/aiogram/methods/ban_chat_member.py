from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, DateTimeUnion
from .base import TelegramMethod


class BanChatMember(TelegramMethod[bool]):
    """
    Use this method to ban a user in a group, a supergroup or a channel. In the case of supergroups and channels, the user will not be able to return to the chat on their own using invite links, etc., unless `unbanned <https://core.telegram.org/bots/api#unbanchatmember>`_ first. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#banchatmember
    """

    __returning__ = bool
    __api_method__ = "banChatMember"

    chat_id: ChatIdUnion
    """Unique identifier for the target group or username of the target supergroup or channel in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""
    until_date: DateTimeUnion | None = None
    """Date when the user will be unbanned; Unix time. If user is banned for more than 366 days or less than 30 seconds from the current time they are considered to be banned forever. Applied for supergroups and channels only"""
    revoke_messages: bool | None = None
    """Pass :code:`True` to delete all messages from the chat for the user that is being removed. If :code:`False`, the user will be able to see messages in the group that were sent before the user was removed. Always :code:`True` for supergroups and channels"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int,
            until_date: DateTimeUnion | None = None,
            revoke_messages: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                user_id=user_id,
                until_date=until_date,
                revoke_messages=revoke_messages,
                **__pydantic_kwargs,
            )
