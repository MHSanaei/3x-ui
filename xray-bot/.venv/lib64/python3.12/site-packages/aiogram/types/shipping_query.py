from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import AnswerShippingQuery
    from ..types import ShippingOption
    from .shipping_address import ShippingAddress
    from .user import User


class ShippingQuery(TelegramObject):
    """
    This object contains information about an incoming shipping query.

    Source: https://core.telegram.org/bots/api#shippingquery
    """

    id: str
    """Unique query identifier"""
    from_user: User = Field(..., alias="from")
    """User who sent the query"""
    invoice_payload: str
    """Bot-specified invoice payload"""
    shipping_address: ShippingAddress
    """User specified shipping address"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            from_user: User,
            invoice_payload: str,
            shipping_address: ShippingAddress,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                from_user=from_user,
                invoice_payload=invoice_payload,
                shipping_address=shipping_address,
                **__pydantic_kwargs,
            )

    def answer(
        self,
        ok: bool,
        shipping_options: list[ShippingOption] | None = None,
        error_message: str | None = None,
        **kwargs: Any,
    ) -> AnswerShippingQuery:
        """
        Shortcut for method :class:`aiogram.methods.answer_shipping_query.AnswerShippingQuery`
        will automatically fill method attributes:

        - :code:`shipping_query_id`

        If you sent an invoice requesting a shipping address and the parameter *is_flexible* was specified, the Bot API will send an :class:`aiogram.types.update.Update` with a *shipping_query* field to the bot. Use this method to reply to shipping queries. On success, :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#answershippingquery

        :param ok: Pass :code:`True` if delivery to the specified address is possible and :code:`False` if there are any problems (for example, if delivery to the specified address is not possible)
        :param shipping_options: Required if *ok* is :code:`True`. A JSON-serialized array of available shipping options
        :param error_message: Required if *ok* is :code:`False`. Error message in human readable form that explains why it is impossible to complete the order (e.g. 'Sorry, delivery to your desired address is unavailable'). Telegram will display this message to the user
        :return: instance of method :class:`aiogram.methods.answer_shipping_query.AnswerShippingQuery`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import AnswerShippingQuery

        return AnswerShippingQuery(
            shipping_query_id=self.id,
            ok=ok,
            shipping_options=shipping_options,
            error_message=error_message,
            **kwargs,
        ).as_(self._bot)
