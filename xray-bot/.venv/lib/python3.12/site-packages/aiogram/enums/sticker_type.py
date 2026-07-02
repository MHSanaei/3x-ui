from enum import Enum


class StickerType(str, Enum):
    """
    The part of the face relative to which the mask should be placed.

    Source: https://core.telegram.org/bots/api#maskposition
    """

    REGULAR = "regular"
    MASK = "mask"
    CUSTOM_EMOJI = "custom_emoji"
