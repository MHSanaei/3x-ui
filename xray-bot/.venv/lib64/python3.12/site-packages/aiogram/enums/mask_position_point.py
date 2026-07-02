from enum import Enum


class MaskPositionPoint(str, Enum):
    """
    The part of the face relative to which the mask should be placed.

    Source: https://core.telegram.org/bots/api#maskposition
    """

    FOREHEAD = "forehead"
    EYES = "eyes"
    MOUTH = "mouth"
    CHIN = "chin"
