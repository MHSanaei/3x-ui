from enum import Enum


class InputMediaType(str, Enum):
    """
    This object represents input media type

    Source: https://core.telegram.org/bots/api#inputmedia
    """

    ANIMATION = "animation"
    AUDIO = "audio"
    DOCUMENT = "document"
    PHOTO = "photo"
    VIDEO = "video"
    LIVE_PHOTO = "live_photo"
    VENUE = "venue"
    STICKER = "sticker"
    LOCATION = "location"
