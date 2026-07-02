from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class RefundStarPayment(TelegramMethod[bool]):
    """
    Refunds a successful payment in `Telegram Stars <https://t.me/BotNews/90>`_. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#refundstarpayment
    """

    __returning__ = bool
    __api_method__ = "refundStarPayment"

    user_id: int
    """Identifier of the user whose payment will be refunded"""
    telegram_payment_charge_id: str
    """Telegram payment identifier"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            telegram_payment_charge_id: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                telegram_payment_charge_id=telegram_payment_charge_id,
                **__pydantic_kwargs,
            )
