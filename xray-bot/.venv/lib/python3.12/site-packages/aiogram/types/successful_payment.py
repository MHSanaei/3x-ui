from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .order_info import OrderInfo


class SuccessfulPayment(TelegramObject):
    """
    This object contains basic information about a successful payment. Note that if the buyer initiates a chargeback with the relevant payment provider following this transaction, the funds may be debited from your balance. This is outside of Telegram's control.

    Source: https://core.telegram.org/bots/api#successfulpayment
    """

    currency: str
    """Three-letter ISO 4217 `currency <https://core.telegram.org/bots/payments#supported-currencies>`_ code, or 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    total_amount: int
    """Total price in the *smallest units* of the currency (integer, **not** float/double). For example, for a price of :code:`US$ 1.45` pass :code:`amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies)"""
    invoice_payload: str
    """Bot-specified invoice payload"""
    telegram_payment_charge_id: str
    """Telegram payment identifier"""
    provider_payment_charge_id: str
    """Provider payment identifier"""
    subscription_expiration_date: int | None = None
    """*Optional*. Expiration date of the subscription, in Unix time; for recurring payments only"""
    is_recurring: bool | None = None
    """*Optional*. :code:`True`, if the payment is a recurring payment for a subscription"""
    is_first_recurring: bool | None = None
    """*Optional*. :code:`True`, if the payment is the first payment for a subscription"""
    shipping_option_id: str | None = None
    """*Optional*. Identifier of the shipping option chosen by the user"""
    order_info: OrderInfo | None = None
    """*Optional*. Order information provided by the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            currency: str,
            total_amount: int,
            invoice_payload: str,
            telegram_payment_charge_id: str,
            provider_payment_charge_id: str,
            subscription_expiration_date: int | None = None,
            is_recurring: bool | None = None,
            is_first_recurring: bool | None = None,
            shipping_option_id: str | None = None,
            order_info: OrderInfo | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                currency=currency,
                total_amount=total_amount,
                invoice_payload=invoice_payload,
                telegram_payment_charge_id=telegram_payment_charge_id,
                provider_payment_charge_id=provider_payment_charge_id,
                subscription_expiration_date=subscription_expiration_date,
                is_recurring=is_recurring,
                is_first_recurring=is_first_recurring,
                shipping_option_id=shipping_option_id,
                order_info=order_info,
                **__pydantic_kwargs,
            )
