from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class UnbanChatMember(TelegramMethod[bool]):
    """
    Use this method to unban a previously banned user in a supergroup or channel. The user will **not** return to the group or channel automatically, but will be able to join via link, etc. The bot must be an administrator for this to work. By default, this method guarantees that after the call the user is not a member of the chat, but will be able to join it. So if the user is a member of the chat they will also be **removed** from the chat. If you don't want this, use the parameter *only_if_banned*. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#unbanchatmember
    """

    __returning__ = bool
    __api_method__ = "unbanChatMember"

    chat_id: ChatIdUnion
    """Unique identifier for the target group or username of the target supergroup or channel in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""
    only_if_banned: bool | None = None
    """Do nothing if the user is not banned"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int,
            only_if_banned: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                user_id=user_id,
                only_if_banned=only_if_banned,
                **__pydantic_kwargs,
            )
