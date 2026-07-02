from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class SetChatMemberTag(TelegramMethod[bool]):
    """
    Use this method to set a tag for a regular member in a group or a supergroup. The bot must be an administrator in the chat for this to work and must have the *can_manage_tags* administrator right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setchatmembertag
    """

    __returning__ = bool
    __api_method__ = "setChatMemberTag"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""
    tag: str | None = None
    """New tag for the member; 0-16 characters, emoji are not allowed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int,
            tag: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, user_id=user_id, tag=tag, **__pydantic_kwargs)
