from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import AnswerPreCheckoutQuery
    from .order_info import OrderInfo
    from .user import User


class PreCheckoutQuery(TelegramObject):
    """
    This object contains information about an incoming pre-checkout query.

    Source: https://core.telegram.org/bots/api#precheckoutquery
    """

    id: str
    """Unique query identifier"""
    from_user: User = Field(..., alias="from")
    """User who sent the query"""
    currency: str
    """Three-letter ISO 4217 `currency <https://core.telegram.org/bots/payments#supported-currencies>`_ code, or 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    total_amount: int
    """Total price in the *smallest units* of the currency (integer, **not** float/double). For example, for a price of :code:`US$ 1.45` pass :code:`amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies)"""
    invoice_payload: str
    """Bot-specified invoice payload"""
    shipping_option_id: str | None = None
    """*Optional*. Identifier of the shipping option chosen by the user"""
    order_info: OrderInfo | None = None
    """*Optional*. Order information provided by the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            from_user: User,
            currency: str,
            total_amount: int,
            invoice_payload: str,
            shipping_option_id: str | None = None,
            order_info: OrderInfo | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                from_user=from_user,
                currency=currency,
                total_amount=total_amount,
                invoice_payload=invoice_payload,
                shipping_option_id=shipping_option_id,
                order_info=order_info,
                **__pydantic_kwargs,
            )

    def answer(
        self,
        ok: bool,
        error_message: str | None = None,
        **kwargs: Any,
    ) -> AnswerPreCheckoutQuery:
        """
        Shortcut for method :class:`aiogram.methods.answer_pre_checkout_query.AnswerPreCheckoutQuery`
        will automatically fill method attributes:

        - :code:`pre_checkout_query_id`

        Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form of an :class:`aiogram.types.update.Update` with the field *pre_checkout_query*. Use this method to respond to such pre-checkout queries. On success, :code:`True` is returned. **Note:** The Bot API must receive an answer within 10 seconds after the pre-checkout query was sent.

        Source: https://core.telegram.org/bots/api#answerprecheckoutquery

        :param ok: Specify :code:`True` if everything is alright (goods are available, etc.) and the bot is ready to proceed with the order. Use :code:`False` if there are any problems
        :param error_message: Required if *ok* is :code:`False`. Error message in human readable form that explains the reason for failure to proceed with the checkout (e.g. "Sorry, somebody just bought the last of our amazing black T-shirts while you were busy filling out your payment details. Please choose a different color or garment!"). Telegram will display this message to the user
        :return: instance of method :class:`aiogram.methods.answer_pre_checkout_query.AnswerPreCheckoutQuery`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import AnswerPreCheckoutQuery

        return AnswerPreCheckoutQuery(
            pre_checkout_query_id=self.id,
            ok=ok,
            error_message=error_message,
            **kwargs,
        ).as_(self._bot)
