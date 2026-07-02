from typing import TypeAlias

from .story_area_type_link import StoryAreaTypeLink
from .story_area_type_location import StoryAreaTypeLocation
from .story_area_type_suggested_reaction import StoryAreaTypeSuggestedReaction
from .story_area_type_unique_gift import StoryAreaTypeUniqueGift
from .story_area_type_weather import StoryAreaTypeWeather

StoryAreaTypeUnion: TypeAlias = (
    StoryAreaTypeLocation
    | StoryAreaTypeSuggestedReaction
    | StoryAreaTypeLink
    | StoryAreaTypeWeather
    | StoryAreaTypeUniqueGift
)
