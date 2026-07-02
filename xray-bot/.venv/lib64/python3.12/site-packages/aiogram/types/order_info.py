from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .shipping_address import ShippingAddress


class OrderInfo(TelegramObject):
    """
    This object represents information about an order.

    Source: https://core.telegram.org/bots/api#orderinfo
    """

    name: str | None = None
    """*Optional*. User name"""
    phone_number: str | None = None
    """*Optional*. User's phone number"""
    email: str | None = None
    """*Optional*. User email"""
    shipping_address: ShippingAddress | None = None
    """*Optional*. User shipping address"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str | None = None,
            phone_number: str | None = None,
            email: str | None = None,
            shipping_address: ShippingAddress | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                name=name,
                phone_number=phone_number,
                email=email,
                shipping_address=shipping_address,
                **__pydantic_kwargs,
            )
