from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class SetChatAdministratorCustomTitle(TelegramMethod[bool]):
    """
    Use this method to set a custom title for an administrator in a supergroup promoted by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setchatadministratorcustomtitle
    """

    __returning__ = bool
    __api_method__ = "setChatAdministratorCustomTitle"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""
    custom_title: str
    """New custom title for the administrator; 0-16 characters, emoji are not allowed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int,
            custom_title: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, user_id=user_id, custom_title=custom_title, **__pydantic_kwargs
            )
