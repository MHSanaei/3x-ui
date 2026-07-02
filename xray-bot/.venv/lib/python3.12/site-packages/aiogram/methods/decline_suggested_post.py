from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class DeclineSuggestedPost(TelegramMethod[bool]):
    """
    Use this method to decline a suggested post in a direct messages chat. The bot must have the 'can_manage_direct_messages' administrator right in the corresponding channel chat. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#declinesuggestedpost
    """

    __returning__ = bool
    __api_method__ = "declineSuggestedPost"

    chat_id: int
    """Unique identifier for the target direct messages chat"""
    message_id: int
    """Identifier of a suggested post message to decline"""
    comment: str | None = None
    """Comment for the creator of the suggested post; 0-128 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: int,
            message_id: int,
            comment: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, message_id=message_id, comment=comment, **__pydantic_kwargs
            )
