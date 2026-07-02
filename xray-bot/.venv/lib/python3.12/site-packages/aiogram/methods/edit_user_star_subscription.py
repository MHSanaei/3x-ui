from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class EditUserStarSubscription(TelegramMethod[bool]):
    """
    Allows the bot to cancel or re-enable extension of a subscription paid in Telegram Stars. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#edituserstarsubscription
    """

    __returning__ = bool
    __api_method__ = "editUserStarSubscription"

    user_id: int
    """Identifier of the user whose subscription will be edited"""
    telegram_payment_charge_id: str
    """Telegram payment identifier for the subscription"""
    is_canceled: bool
    """Pass :code:`True` to cancel extension of the user subscription; the subscription must be active up to the end of the current subscription period. Pass :code:`False` to allow the user to re-enable a subscription that was previously canceled by the bot"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            telegram_payment_charge_id: str,
            is_canceled: bool,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                telegram_payment_charge_id=telegram_payment_charge_id,
                is_canceled=is_canceled,
                **__pydantic_kwargs,
            )
