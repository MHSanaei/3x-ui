from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import DateTimeUnion
from .base import TelegramMethod


class ApproveSuggestedPost(TelegramMethod[bool]):
    """
    Use this method to approve a suggested post in a direct messages chat. The bot must have the 'can_post_messages' administrator right in the corresponding channel chat. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#approvesuggestedpost
    """

    __returning__ = bool
    __api_method__ = "approveSuggestedPost"

    chat_id: int
    """Unique identifier for the target direct messages chat"""
    message_id: int
    """Identifier of a suggested post message to approve"""
    send_date: DateTimeUnion | None = None
    """Point in time (Unix timestamp) when the post is expected to be published; omit if the date has already been specified when the suggested post was created. If specified, then the date must be not more than 2678400 seconds (30 days) in the future"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: int,
            message_id: int,
            send_date: DateTimeUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, message_id=message_id, send_date=send_date, **__pydantic_kwargs
            )
