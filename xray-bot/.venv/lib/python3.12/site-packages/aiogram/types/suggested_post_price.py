from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class SuggestedPostPrice(TelegramObject):
    """
    Describes the price of a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostprice
    """

    currency: str
    """Currency in which the post will be paid. Currently, must be one of 'XTR' for Telegram Stars or 'TON' for toncoins"""
    amount: int
    """The amount of the currency that will be paid for the post in the *smallest units* of the currency, i.e. Telegram Stars or nanotoncoins. Currently, price in Telegram Stars must be between 5 and 100000, and price in nanotoncoins must be between 10000000 and 10000000000000"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, currency: str, amount: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(currency=currency, amount=amount, **__pydantic_kwargs)
