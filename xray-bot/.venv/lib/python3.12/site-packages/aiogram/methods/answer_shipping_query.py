from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ShippingOption
from .base import TelegramMethod


class AnswerShippingQuery(TelegramMethod[bool]):
    """
    If you sent an invoice requesting a shipping address and the parameter *is_flexible* was specified, the Bot API will send an :class:`aiogram.types.update.Update` with a *shipping_query* field to the bot. Use this method to reply to shipping queries. On success, :code:`True` is returned.

    Source: https://core.telegram.org/bots/api#answershippingquery
    """

    __returning__ = bool
    __api_method__ = "answerShippingQuery"

    shipping_query_id: str
    """Unique identifier for the query to be answered"""
    ok: bool
    """Pass :code:`True` if delivery to the specified address is possible and :code:`False` if there are any problems (for example, if delivery to the specified address is not possible)"""
    shipping_options: list[ShippingOption] | None = None
    """Required if *ok* is :code:`True`. A JSON-serialized array of available shipping options"""
    error_message: str | None = None
    """Required if *ok* is :code:`False`. Error message in human readable form that explains why it is impossible to complete the order (e.g. 'Sorry, delivery to your desired address is unavailable'). Telegram will display this message to the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            shipping_query_id: str,
            ok: bool,
            shipping_options: list[ShippingOption] | None = None,
            error_message: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                shipping_query_id=shipping_query_id,
                ok=ok,
                shipping_options=shipping_options,
                error_message=error_message,
                **__pydantic_kwargs,
            )
