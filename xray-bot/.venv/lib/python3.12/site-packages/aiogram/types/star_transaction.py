from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .transaction_partner_union import TransactionPartnerUnion


class StarTransaction(TelegramObject):
    """
    Describes a Telegram Star transaction. Note that if the buyer initiates a chargeback with the payment provider from whom they acquired Stars (e.g., Apple, Google) following this transaction, the refunded Stars will be deducted from the bot's balance. This is outside of Telegram's control.

    Source: https://core.telegram.org/bots/api#startransaction
    """

    id: str
    """Unique identifier of the transaction. Coincides with the identifier of the original transaction for refund transactions. Coincides with *SuccessfulPayment.telegram_payment_charge_id* for successful incoming payments from users"""
    amount: int
    """Integer amount of Telegram Stars transferred by the transaction"""
    date: DateTime
    """Date the transaction was created in Unix time"""
    nanostar_amount: int | None = None
    """*Optional*. The number of 1/1000000000 shares of Telegram Stars transferred by the transaction; from 0 to 999999999"""
    source: TransactionPartnerUnion | None = None
    """*Optional*. Source of an incoming transaction (e.g., a user purchasing goods or services, Fragment refunding a failed withdrawal). Only for incoming transactions"""
    receiver: TransactionPartnerUnion | None = None
    """*Optional*. Receiver of an outgoing transaction (e.g., a user for a purchase refund, Fragment for a withdrawal). Only for outgoing transactions"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            amount: int,
            date: DateTime,
            nanostar_amount: int | None = None,
            source: TransactionPartnerUnion | None = None,
            receiver: TransactionPartnerUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                amount=amount,
                date=date,
                nanostar_amount=nanostar_amount,
                source=source,
                receiver=receiver,
                **__pydantic_kwargs,
            )
