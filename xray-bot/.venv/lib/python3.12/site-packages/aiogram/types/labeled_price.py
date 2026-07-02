from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject


class LabeledPrice(MutableTelegramObject):
    """
    This object represents a portion of the price for goods or services.

    Source: https://core.telegram.org/bots/api#labeledprice
    """

    label: str
    """Portion label"""
    amount: int
    """Price of the product in the *smallest units* of the `currency <https://core.telegram.org/bots/payments#supported-currencies>`_ (integer, **not** float/double). For example, for a price of :code:`US$ 1.45` pass :code:`amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies)"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, label: str, amount: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(label=label, amount=amount, **__pydantic_kwargs)
