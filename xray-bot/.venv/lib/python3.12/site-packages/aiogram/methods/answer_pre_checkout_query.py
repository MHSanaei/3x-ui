from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class AnswerPreCheckoutQuery(TelegramMethod[bool]):
    """
    Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form of an :class:`aiogram.types.update.Update` with the field *pre_checkout_query*. Use this method to respond to such pre-checkout queries. On success, :code:`True` is returned. **Note:** The Bot API must receive an answer within 10 seconds after the pre-checkout query was sent.

    Source: https://core.telegram.org/bots/api#answerprecheckoutquery
    """

    __returning__ = bool
    __api_method__ = "answerPreCheckoutQuery"

    pre_checkout_query_id: str
    """Unique identifier for the query to be answered"""
    ok: bool
    """Specify :code:`True` if everything is alright (goods are available, etc.) and the bot is ready to proceed with the order. Use :code:`False` if there are any problems"""
    error_message: str | None = None
    """Required if *ok* is :code:`False`. Error message in human readable form that explains the reason for failure to proceed with the checkout (e.g. "Sorry, somebody just bought the last of our amazing black T-shirts while you were busy filling out your payment details. Please choose a different color or garment!"). Telegram will display this message to the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            pre_checkout_query_id: str,
            ok: bool,
            error_message: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                pre_checkout_query_id=pre_checkout_query_id,
                ok=ok,
                error_message=error_message,
                **__pydantic_kwargs,
            )
