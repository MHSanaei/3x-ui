from enum import Enum


class TransactionPartnerUserTransactionTypeEnum(str, Enum):
    """
    This object represents type of the transaction that were made by partner user.

    Source: https://core.telegram.org/bots/api#transactionpartneruser
    """

    INVOICE_PAYMENT = "invoice_payment"
    PAID_MEDIA_PAYMENT = "paid_media_payment"
    GIFT_PURCHASE = "gift_purchase"
    PREMIUM_PURCHASE = "premium_purchase"
    BUSINESS_ACCOUNT_TRANSFER = "business_account_transfer"
