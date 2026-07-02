from __future__ import annotations

from typing import TypeAlias

from .transaction_partner_affiliate_program import TransactionPartnerAffiliateProgram
from .transaction_partner_chat import TransactionPartnerChat
from .transaction_partner_fragment import TransactionPartnerFragment
from .transaction_partner_other import TransactionPartnerOther
from .transaction_partner_telegram_ads import TransactionPartnerTelegramAds
from .transaction_partner_telegram_api import TransactionPartnerTelegramApi
from .transaction_partner_user import TransactionPartnerUser

TransactionPartnerUnion: TypeAlias = (
    TransactionPartnerUser
    | TransactionPartnerChat
    | TransactionPartnerAffiliateProgram
    | TransactionPartnerFragment
    | TransactionPartnerTelegramAds
    | TransactionPartnerTelegramApi
    | TransactionPartnerOther
)
