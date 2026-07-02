from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner

if TYPE_CHECKING:
    from .affiliate_info import AffiliateInfo
    from .gift import Gift
    from .paid_media_union import PaidMediaUnion
    from .user import User


class TransactionPartnerUser(TransactionPartner):
    """
    Describes a transaction with a user.

    Source: https://core.telegram.org/bots/api#transactionpartneruser
    """

    type: Literal[TransactionPartnerType.USER] = TransactionPartnerType.USER
    """Type of the transaction partner, always 'user'"""
    transaction_type: str
    """Type of the transaction, currently one of 'invoice_payment' for payments via invoices, 'paid_media_payment' for payments for paid media, 'gift_purchase' for gifts sent by the bot, 'premium_purchase' for Telegram Premium subscriptions gifted by the bot, 'business_account_transfer' for direct transfers from managed business accounts"""
    user: User
    """Information about the user"""
    affiliate: AffiliateInfo | None = None
    """*Optional*. Information about the affiliate that received a commission via this transaction. Can be available only for 'invoice_payment' and 'paid_media_payment' transactions"""
    invoice_payload: str | None = None
    """*Optional*. Bot-specified invoice payload. Can be available only for 'invoice_payment' transactions"""
    subscription_period: int | None = None
    """*Optional*. The duration of the paid subscription. Can be available only for 'invoice_payment' transactions"""
    paid_media: list[PaidMediaUnion] | None = None
    """*Optional*. Information about the paid media bought by the user; for 'paid_media_payment' transactions only"""
    paid_media_payload: str | None = None
    """*Optional*. Bot-specified paid media payload. Can be available only for 'paid_media_payment' transactions"""
    gift: Gift | None = None
    """*Optional*. The gift sent to the user by the bot; for 'gift_purchase' transactions only"""
    premium_subscription_duration: int | None = None
    """*Optional*. Number of months the gifted Telegram Premium subscription will be active for; for 'premium_purchase' transactions only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[TransactionPartnerType.USER] = TransactionPartnerType.USER,
            transaction_type: str,
            user: User,
            affiliate: AffiliateInfo | None = None,
            invoice_payload: str | None = None,
            subscription_period: int | None = None,
            paid_media: list[PaidMediaUnion] | None = None,
            paid_media_payload: str | None = None,
            gift: Gift | None = None,
            premium_subscription_duration: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                transaction_type=transaction_type,
                user=user,
                affiliate=affiliate,
                invoice_payload=invoice_payload,
                subscription_period=subscription_period,
                paid_media=paid_media,
                paid_media_payload=paid_media_payload,
                gift=gift,
                premium_subscription_duration=premium_subscription_duration,
                **__pydantic_kwargs,
            )
