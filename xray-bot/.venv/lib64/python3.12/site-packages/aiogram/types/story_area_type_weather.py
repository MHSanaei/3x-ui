from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import StoryAreaTypeType

from .story_area_type import StoryAreaType


class StoryAreaTypeWeather(StoryAreaType):
    """
    Describes a story area containing weather information. Currently, a story can have up to 3 weather areas.

    Source: https://core.telegram.org/bots/api#storyareatypeweather
    """

    type: Literal[StoryAreaTypeType.WEATHER] = StoryAreaTypeType.WEATHER
    """Type of the area, always 'weather'"""
    temperature: float
    """Temperature, in degree Celsius"""
    emoji: str
    """Emoji representing the weather"""
    background_color: int
    """A color of the area background in the ARGB format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[StoryAreaTypeType.WEATHER] = StoryAreaTypeType.WEATHER,
            temperature: float,
            emoji: str,
            background_color: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                temperature=temperature,
                emoji=emoji,
                background_color=background_color,
                **__pydantic_kwargs,
            )
