from enum import Enum


class ChatBoostSourceType(str, Enum):
    """
    This object represents a type of chat boost source.

    Source: https://core.telegram.org/bots/api#chatboostsource
    """

    PREMIUM = "premium"
    GIFT_CODE = "gift_code"
    GIVEAWAY = "giveaway"
