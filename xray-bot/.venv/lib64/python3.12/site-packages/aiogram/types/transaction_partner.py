from .base import TelegramObject


class TransactionPartner(TelegramObject):
    """
    This object describes the source of a transaction, or its recipient for outgoing transactions. Currently, it can be one of

     - :class:`aiogram.types.transaction_partner_user.TransactionPartnerUser`
     - :class:`aiogram.types.transaction_partner_chat.TransactionPartnerChat`
     - :class:`aiogram.types.transaction_partner_affiliate_program.TransactionPartnerAffiliateProgram`
     - :class:`aiogram.types.transaction_partner_fragment.TransactionPartnerFragment`
     - :class:`aiogram.types.transaction_partner_telegram_ads.TransactionPartnerTelegramAds`
     - :class:`aiogram.types.transaction_partner_telegram_api.TransactionPartnerTelegramApi`
     - :class:`aiogram.types.transaction_partner_other.TransactionPartnerOther`

    Source: https://core.telegram.org/bots/api#transactionpartner
    """
