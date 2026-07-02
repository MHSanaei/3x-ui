from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class ReopenGeneralForumTopic(TelegramMethod[bool]):
    """
    Use this method to reopen a closed 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. The topic will be automatically unhidden if it was hidden. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#reopengeneralforumtopic
    """

    __returning__ = bool
    __api_method__ = "reopenGeneralForumTopic"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""

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
