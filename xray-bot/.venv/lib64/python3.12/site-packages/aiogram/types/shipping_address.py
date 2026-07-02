from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class ShippingAddress(TelegramObject):
    """
    This object represents a shipping address.

    Source: https://core.telegram.org/bots/api#shippingaddress
    """

    country_code: str
    """Two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country code"""
    state: str
    """State, if applicable"""
    city: str
    """City"""
    street_line1: str
    """First line for the address"""
    street_line2: str
    """Second line for the address"""
    post_code: str
    """Address post code"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            country_code: str,
            state: str,
            city: str,
            street_line1: str,
            street_line2: str,
            post_code: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                country_code=country_code,
                state=state,
                city=city,
                street_line1=street_line1,
                street_line2=street_line2,
                post_code=post_code,
                **__pydantic_kwargs,
            )
