from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat


class Story(TelegramObject):
    """
    This object represents a story.

    Source: https://core.telegram.org/bots/api#story
    """

    chat: Chat
    """Chat that posted the story"""
    id: int
    """Unique identifier for the story in the chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, chat: Chat, id: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat=chat, id=id, **__pydantic_kwargs)
