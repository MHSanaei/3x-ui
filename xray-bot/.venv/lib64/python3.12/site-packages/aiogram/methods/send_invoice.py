from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from ..types import (
    ChatIdUnion,
    InlineKeyboardMarkup,
    LabeledPrice,
    Message,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class SendInvoice(TelegramMethod[Message]):
    """
    Use this method to send invoices. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendinvoice
    """

    __returning__ = Message
    __api_method__ = "sendInvoice"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
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
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
    provider_token: str | None = None
    """Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    max_tip_amount: int | None = None
    """The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    suggested_tip_amounts: list[int] | None = None
    """A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*"""
    start_parameter: str | None = None
    """Unique deep-linking parameter. If left empty, **forwarded copies** of the sent message will have a *Pay* button, allowing multiple users to pay directly from the forwarded message, using the same invoice. If non-empty, forwarded copies of the sent message will have a *URL* button with a deep link to the bot (instead of a *Pay* button), with the value used as the start parameter"""
    provider_data: str | None = None
    """JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider"""
    photo_url: str | None = None
    """URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service. People like it better when they see what they are paying for"""
    photo_size: int | None = None
    """Photo size in bytes"""
    photo_width: int | None = None
    """Photo width"""
    photo_height: int | None = None
    """Photo height"""
    need_name: bool | None = None
    """Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_phone_number: bool | None = None
    """Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_email: bool | None = None
    """Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    need_shipping_address: bool | None = None
    """Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    send_phone_number_to_provider: bool | None = None
    """Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    send_email_to_provider: bool | None = None
    """Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    is_flexible: bool | None = None
    """Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_"""
    disable_notification: bool | None = None
    """Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound"""
    protect_content: bool | Default | None = Default("protect_content")
    """Protects the contents of the sent message from forwarding and saving"""
    allow_paid_broadcast: bool | None = None
    """Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance"""
    message_effect_id: str | None = None
    """Unique identifier of the message effect to be added to the message; for private chats only"""
    suggested_post_parameters: SuggestedPostParameters | None = None
    """A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined"""
    reply_parameters: ReplyParameters | None = None
    """Description of the message to reply to"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Pay :code:`total price`' button will be shown. If not empty, the first button must be a Pay button"""
    allow_sending_without_reply: bool | None = Field(None, json_schema_extra={"deprecated": True})
    """Pass :code:`True` if the message should be sent even if the specified replied-to message is not found

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    reply_to_message_id: int | None = Field(None, json_schema_extra={"deprecated": True})
    """If the message is a reply, ID of the original message

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            title: str,
            description: str,
            payload: str,
            currency: str,
            prices: list[LabeledPrice],
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
            provider_token: str | None = None,
            max_tip_amount: int | None = None,
            suggested_tip_amounts: list[int] | None = None,
            start_parameter: str | None = None,
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
            disable_notification: bool | None = None,
            protect_content: bool | Default | None = Default("protect_content"),
            allow_paid_broadcast: bool | None = None,
            message_effect_id: str | None = None,
            suggested_post_parameters: SuggestedPostParameters | None = None,
            reply_parameters: ReplyParameters | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            allow_sending_without_reply: bool | None = None,
            reply_to_message_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                title=title,
                description=description,
                payload=payload,
                currency=currency,
                prices=prices,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                provider_token=provider_token,
                max_tip_amount=max_tip_amount,
                suggested_tip_amounts=suggested_tip_amounts,
                start_parameter=start_parameter,
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
                disable_notification=disable_notification,
                protect_content=protect_content,
                allow_paid_broadcast=allow_paid_broadcast,
                message_effect_id=message_effect_id,
                suggested_post_parameters=suggested_post_parameters,
                reply_parameters=reply_parameters,
                reply_markup=reply_markup,
                allow_sending_without_reply=allow_sending_without_reply,
                reply_to_message_id=reply_to_message_id,
                **__pydantic_kwargs,
            )
