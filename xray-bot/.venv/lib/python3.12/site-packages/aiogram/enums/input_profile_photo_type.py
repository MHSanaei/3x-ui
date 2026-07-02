from enum import Enum


class InputProfilePhotoType(str, Enum):
    """
    This object represents input profile photo type

    Source: https://core.telegram.org/bots/api#inputprofilephoto
    """

    STATIC = "static"
    ANIMATED = "animated"
