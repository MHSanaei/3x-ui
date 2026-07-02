from enum import Enum


class TransactionPartnerType(str, Enum):
    """
    This object represents a type of transaction partner.

    Source: https://core.telegram.org/bots/api#transactionpartner
    """

    FRAGMENT = "fragment"
    OTHER = "other"
    USER = "user"
    TELEGRAM_ADS = "telegram_ads"
    TELEGRAM_API = "telegram_api"
    AFFILIATE_PROGRAM = "affiliate_program"
    CHAT = "chat"
