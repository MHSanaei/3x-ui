from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from ..types import (
    ChatIdUnion,
    DateTimeUnion,
    MessageEntity,
    MessageId,
    ReplyMarkupUnion,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class CopyMessage(TelegramMethod[MessageId]):
    """
    Use this method to copy messages of any kind. Service messages, paid media messages, giveaway messages, giveaway winners messages, and invoice messages can't be copied. A quiz :class:`aiogram.methods.poll.Poll` can be copied only if the value of the field *correct_option_id* is known to the bot. The method is analogous to the method :class:`aiogram.methods.forward_message.ForwardMessage`, but the copied message doesn't have a link to the original message. Returns the :class:`aiogram.types.message_id.MessageId` of the sent message on success.

    Source: https://core.telegram.org/bots/api#copymessage
    """

    __returning__ = MessageId
    __api_method__ = "copyMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    from_chat_id: ChatIdUnion
    """Unique identifier for the chat where the original message was sent (or username of the target bot, supergroup or channel in the format :code:`@username`)"""
    message_id: int
    """Message identifier in the chat specified in *from_chat_id*"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
    video_start_timestamp: DateTimeUnion | None = None
    """New start timestamp for the copied video in the message"""
    caption: str | None = None
    """New caption for media, 0-1024 characters after entities parsing. If not specified, the original caption is kept"""
    parse_mode: str | Default | None = Default("parse_mode")
    """Mode for parsing entities in the new caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the new caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | Default | None = Default("show_caption_above_media")
    """Pass :code:`True`, if the caption must be shown above the message media. Ignored if a new caption isn't specified"""
    disable_notification: bool | None = None
    """Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound"""
    protect_content: bool | Default | None = Default("protect_content")
    """Protects the contents of the sent message from forwarding and saving"""
    allow_paid_broadcast: bool | None = None
    """Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance"""
    message_effect_id: str | None = None
    """Unique identifier of the message effect to be added to the message; only available when copying to private chats"""
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
            from_chat_id: ChatIdUnion,
            message_id: int,
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
            video_start_timestamp: DateTimeUnion | None = None,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
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
                from_chat_id=from_chat_id,
                message_id=message_id,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                video_start_timestamp=video_start_timestamp,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
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
