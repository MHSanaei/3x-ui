from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import StoryAreaTypeType

from .story_area_type import StoryAreaType


class StoryAreaTypeUniqueGift(StoryAreaType):
    """
    Describes a story area pointing to a unique gift. Currently, a story can have at most 1 unique gift area.

    Source: https://core.telegram.org/bots/api#storyareatypeuniquegift
    """

    type: Literal[StoryAreaTypeType.UNIQUE_GIFT] = StoryAreaTypeType.UNIQUE_GIFT
    """Type of the area, always 'unique_gift'"""
    name: str
    """Unique name of the gift"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[StoryAreaTypeType.UNIQUE_GIFT] = StoryAreaTypeType.UNIQUE_GIFT,
            name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, name=name, **__pydantic_kwargs)
