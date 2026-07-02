from enum import Enum


class StickerFormat(str, Enum):
    """
    Format of the sticker

    Source: https://core.telegram.org/bots/api#createnewstickerset
    """

    STATIC = "static"
    ANIMATED = "animated"
    VIDEO = "video"
