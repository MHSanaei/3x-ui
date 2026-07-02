from enum import Enum


class OwnedGiftType(str, Enum):
    """
    This object represents owned gift type

    Source: https://core.telegram.org/bots/api#ownedgift
    """

    REGULAR = "regular"
    UNIQUE = "unique"
