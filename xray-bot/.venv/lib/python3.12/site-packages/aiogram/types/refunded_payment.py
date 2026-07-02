from typing import TYPE_CHECKING, Any, Literal

from .base import TelegramObject


class RefundedPayment(TelegramObject):
    """
    This object contains basic information about a refunded payment.

    Source: https://core.telegram.org/bots/api#refundedpayment
    """

    currency: Literal["XTR"] = "XTR"
    """Three-letter ISO 4217 `currency <https://core.telegram.org/bots/payments#supported-currencies>`_ code, or 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_. Currently, always 'XTR'"""
    total_amount: int
    """Total refunded price in the *smallest units* of the currency (integer, **not** float/double). For example, for a price of :code:`US$ 1.45`, :code:`total_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies)"""
    invoice_payload: str
    """Bot-specified invoice payload"""
    telegram_payment_charge_id: str
    """Telegram payment identifier"""
    provider_payment_charge_id: str | None = None
    """*Optional*. Provider payment identifier"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            currency: Literal["XTR"] = "XTR",
            total_amount: int,
            invoice_payload: str,
            telegram_payment_charge_id: str,
            provider_payment_charge_id: str | None = None,
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
                **__pydantic_kwargs,
            )
