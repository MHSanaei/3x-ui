from typing import TYPE_CHECKING, Any

from ..types import Message
from .base import TelegramMethod


class GetUserPersonalChatMessages(TelegramMethod[list[Message]]):
    """
    Use this method to get the last messages from the personal chat (i.e., the chat currently added to their profile) of a given user. On success, an array of :class:`aiogram.types.message.Message` objects is returned.

    Source: https://core.telegram.org/bots/api#getuserpersonalchatmessages
    """

    __returning__ = list[Message]
    __api_method__ = "getUserPersonalChatMessages"

    user_id: int
    """Unique identifier for the target user"""
    limit: int
    """The maximum number of messages to return; 1-20"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, user_id: int, limit: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, limit=limit, **__pydantic_kwargs)
