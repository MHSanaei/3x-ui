from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class Invoice(TelegramObject):
    """
    This object contains basic information about an invoice.

    Source: https://core.telegram.org/bots/api#invoice
    """

    title: str
    """Product name"""
    description: str
    """Product description"""
    start_parameter: str
    """Unique bot deep-linking parameter that can be used to generate this invoice"""
    currency: str
    """Three-letter ISO 4217 `currency <https://core.telegram.org/bots/payments#supported-currencies>`_ code, or 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    total_amount: int
    """Total price in the *smallest units* of the currency (integer, **not** float/double). For example, for a price of :code:`US$ 1.45` pass :code:`amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies)"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str,
            description: str,
            start_parameter: str,
            currency: str,
            total_amount: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                title=title,
                description=description,
                start_parameter=start_parameter,
                currency=currency,
                total_amount=total_amount,
                **__pydantic_kwargs,
            )
