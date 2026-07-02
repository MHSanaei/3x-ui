from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .input_message_content import InputMessageContent

if TYPE_CHECKING:
    from .labeled_price import LabeledPrice


class InputInvoiceMessageContent(InputMessageContent):
    """
    Represents the `content <https://core.telegram.org/bots/api#inputmessagecontent>`_ of an invoice message to be sent as the result of an inline query.

    Source: https://core.telegram.org/bots/api#inputinvoicemessagecontent
    """

    title: str
    """Product name, 1-32 characters"""
    description: str
    """Product description, 1-255 characters"""
    payload: str
    """Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes"""
    currency: str
    """Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    prices: list[LabeledPrice]
    """Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    provider_token: str | None = None
    """*Optional*. Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    max_tip_amount: int | None = None
    """*Optional*. The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    suggested_tip_amounts: list[int] | None = None
    """*Optional*. A JSON-serialized array of suggested amounts of tip in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*"""
    provider_data: str | None = None
    """*Optional*. A JSON-serialized object for data about the invoice, which will be shared with the payment provider. A detailed description of the required fields should be provided by the payment provider"""
    photo_url: str | None = None
    """*Optional*. URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service"""
    photo_size: int | None = None
    """*Optional*. Photo size in bytes"""
    photo_width: int | None = None
    """*Optional*. Photo width"""
    photo_height: int | None = None
    """*Optional*. Photo height"""
    need_name: bool | None = None
    """*Optional*. Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_phone_number: bool | None = None
    """*Optional*. Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_email: bool | None = None
    """*Optional*. Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_shipping_address: bool | None = None
    """*Optional*. Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    send_phone_number_to_provider: bool | None = None
    """*Optional*. Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    send_email_to_provider: bool | None = None
    """*Optional*. Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    is_flexible: bool | None = None
    """*Optional*. Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str,
            description: str,
            payload: str,
            currency: str,
            prices: list[LabeledPrice],
            provider_token: str | None = None,
            max_tip_amount: int | None = None,
            suggested_tip_amounts: list[int] | None = None,
            provider_data: str | None = None,
            photo_url: str | None = None,
            photo_size: int | None = None,
            photo_width: int | None = None,
            photo_height: int | None = None,
            need_name: bool | None = None,
            need_phone_number: bool | None = None,
            need_email: bool | None = None,
            need_shipping_address: bool | None = None,
            send_phone_number_to_provider: bool | None = None,
            send_email_to_provider: bool | None = None,
            is_flexible: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                title=title,
                description=description,
                payload=payload,
                currency=currency,
                prices=prices,
                provider_token=provider_token,
                max_tip_amount=max_tip_amount,
                suggested_tip_amounts=suggested_tip_amounts,
                provider_data=provider_data,
                photo_url=photo_url,
                photo_size=photo_size,
                photo_width=photo_width,
                photo_height=photo_height,
                need_name=need_name,
                need_phone_number=need_phone_number,
                need_email=need_email,
                need_shipping_address=need_shipping_address,
                send_phone_number_to_provider=send_phone_number_to_provider,
                send_email_to_provider=send_email_to_provider,
                is_flexible=is_flexible,
                **__pydantic_kwargs,
            )
