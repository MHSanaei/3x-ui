from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .labeled_price import LabeledPrice


class ShippingOption(TelegramObject):
    """
    This object represents one shipping option.

    Source: https://core.telegram.org/bots/api#shippingoption
    """

    id: str
    """Shipping option identifier"""
    title: str
    """Option title"""
    prices: list[LabeledPrice]
    """List of price portions"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            title: str,
            prices: list[LabeledPrice],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(id=id, title=title, prices=prices, **__pydantic_kwargs)
