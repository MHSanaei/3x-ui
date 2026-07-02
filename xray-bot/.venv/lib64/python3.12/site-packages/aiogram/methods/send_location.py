from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from ..types import (
    ChatIdUnion,
    Message,
    ReplyMarkupUnion,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class SendLocation(TelegramMethod[Message]):
    """
    Use this method to send point on the map. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendlocation
    """

    __returning__ = Message
    __api_method__ = "sendLocation"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    latitude: float
    """Latitude of the location"""
    longitude: float
    """Longitude of the location"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
    horizontal_accuracy: float | None = None
    """The radius of uncertainty for the location, measured in meters; 0-1500"""
    live_period: int | None = None
    """Period in seconds during which the location will be updated (see `Live Locations <https://telegram.org/blog/live-locations>`_, should be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely"""
    heading: int | None = None
    """For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified"""
    proximity_alert_radius: int | None = None
    """For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified"""
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
    reply_markup: ReplyMarkupUnion | None = None
    """Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user"""
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
            latitude: float,
            longitude: float,
            business_connection_id: str | None = None,
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
            horizontal_accuracy: float | None = None,
            live_period: int | None = None,
            heading: int | None = None,
            proximity_alert_radius: int | None = None,
            disable_notification: bool | None = None,
            protect_content: bool | Default | None = Default("protect_content"),
            allow_paid_broadcast: bool | None = None,
            message_effect_id: str | None = None,
            suggested_post_parameters: SuggestedPostParameters | None = None,
            reply_parameters: ReplyParameters | None = None,
            reply_markup: ReplyMarkupUnion | None = None,
            allow_sending_without_reply: bool | None = None,
            reply_to_message_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                latitude=latitude,
                longitude=longitude,
                business_connection_id=business_connection_id,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                horizontal_accuracy=horizontal_accuracy,
                live_period=live_period,
                heading=heading,
                proximity_alert_radius=proximity_alert_radius,
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
