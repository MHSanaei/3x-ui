from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class LeaveChat(TelegramMethod[bool]):
    """
    Use this method for your bot to leave a group, supergroup or channel. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#leavechat
    """

    __returning__ = bool
    __api_method__ = "leaveChat"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`. Channel direct messages chats aren't supported; leave the corresponding channel instead"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat_id: ChatIdUnion, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, **__pydantic_kwargs)
