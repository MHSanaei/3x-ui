from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .story_area_position import StoryAreaPosition
    from .story_area_type_union import StoryAreaTypeUnion


class StoryArea(TelegramObject):
    """
    Describes a clickable area on a story media.

    Source: https://core.telegram.org/bots/api#storyarea
    """

    position: StoryAreaPosition
    """Position of the area"""
    type: StoryAreaTypeUnion
    """Type of the area"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            position: StoryAreaPosition,
            type: StoryAreaTypeUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(position=position, type=type, **__pydantic_kwargs)
