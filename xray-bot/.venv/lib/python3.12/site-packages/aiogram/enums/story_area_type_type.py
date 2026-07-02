from enum import Enum


class StoryAreaTypeType(str, Enum):
    """
    This object represents input profile photo type

    Source: https://core.telegram.org/bots/api#storyareatype
    """

    LOCATION = "location"
    SUGGESTED_REACTION = "suggested_reaction"
    LINK = "link"
    WEATHER = "weather"
    UNIQUE_GIFT = "unique_gift"
