from enum import Enum


class InputPaidMediaType(str, Enum):
    """
    This object represents the type of a media in a paid message.

    Source: https://core.telegram.org/bots/api#inputpaidmedia
    """

    PHOTO = "photo"
    VIDEO = "video"
    LIVE_PHOTO = "live_photo"
