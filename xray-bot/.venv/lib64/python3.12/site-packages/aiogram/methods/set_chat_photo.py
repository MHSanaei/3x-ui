from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InputFile
from .base import TelegramMethod


class SetChatPhoto(TelegramMethod[bool]):
    """
    Use this method to set a new profile photo for the chat. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setchatphoto
    """

    __returning__ = bool
    __api_method__ = "setChatPhoto"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    photo: InputFile
    """New chat photo, uploaded using multipart/form-data"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat_id: ChatIdUnion, photo: InputFile, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, photo=photo, **__pydantic_kwargs)
