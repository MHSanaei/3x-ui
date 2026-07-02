from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class DeleteStory(TelegramMethod[bool]):
    """
    Deletes a story previously posted by the bot on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletestory
    """

    __returning__ = bool
    __api_method__ = "deleteStory"

    business_connection_id: str
    """Unique identifier of the business connection"""
    story_id: int
    """Unique identifier of the story to delete"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            story_id: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                story_id=story_id,
                **__pydantic_kwargs,
            )
