from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class StarAmount(TelegramObject):
    """
    Describes an amount of Telegram Stars.

    Source: https://core.telegram.org/bots/api#staramount
    """

    amount: int
    """Integer amount of Telegram Stars, rounded to 0; can be negative"""
    nanostar_amount: int | None = None
    """*Optional*. The number of 1/1000000000 shares of Telegram Stars; from -999999999 to 999999999; can be negative if and only if *amount* is non-positive"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            amount: int,
            nanostar_amount: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(amount=amount, nanostar_amount=nanostar_amount, **__pydantic_kwargs)
