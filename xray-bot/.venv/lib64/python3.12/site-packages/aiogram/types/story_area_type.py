from __future__ import annotations

from .base import TelegramObject


class StoryAreaType(TelegramObject):
    """
    Describes the type of a clickable area on a story. Currently, it can be one of

     - :class:`aiogram.types.story_area_type_location.StoryAreaTypeLocation`
     - :class:`aiogram.types.story_area_type_suggested_reaction.StoryAreaTypeSuggestedReaction`
     - :class:`aiogram.types.story_area_type_link.StoryAreaTypeLink`
     - :class:`aiogram.types.story_area_type_weather.StoryAreaTypeWeather`
     - :class:`aiogram.types.story_area_type_unique_gift.StoryAreaTypeUniqueGift`

    Source: https://core.telegram.org/bots/api#storyareatype
    """
