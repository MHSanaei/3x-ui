from typing import TYPE_CHECKING, Any

from ..types import (
    ChatIdUnion,
    InputFileUnion,
    Message,
    MessageEntity,
    ReplyMarkupUnion,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class SendLivePhoto(TelegramMethod[Message]):
    """
    Use this method to send live photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendlivephoto
    """

    __returning__ = Message
    __api_method__ = "sendLivePhoto"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel (in the format :code:`@channelusername`)"""
    live_photo: InputFileUnion
    """Live photo video to send. The video must be no longer than 10 seconds and must not exceed 10 MB in size. Pass a file_id as String to send a video that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""
    photo: InputFileUnion
    """The static photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
    caption: str | None = None
    """Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing"""
    parse_mode: str | None = None
    """Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | None = None
    """Pass :code:`True`, if the caption must be shown above the message media"""
    has_spoiler: bool | None = None
    """Pass :code:`True` if the video needs to be covered with a spoiler animation"""
    disable_notification: bool | None = None
    """Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound"""
    protect_content: bool | None = None
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

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            live_photo: InputFileUnion,
            photo: InputFileUnion,
            business_connection_id: str | None = None,
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
            caption: str | None = None,
            parse_mode: str | None = None,
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | None = None,
            has_spoiler: bool | None = None,
            disable_notification: bool | None = None,
            protect_content: bool | None = None,
            allow_paid_broadcast: bool | None = None,
            message_effect_id: str | None = None,
            suggested_post_parameters: SuggestedPostParameters | None = None,
            reply_parameters: ReplyParameters | None = None,
            reply_markup: ReplyMarkupUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                live_photo=live_photo,
                photo=photo,
                business_connection_id=business_connection_id,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                has_spoiler=has_spoiler,
                disable_notification=disable_notification,
                protect_content=protect_content,
                allow_paid_broadcast=allow_paid_broadcast,
                message_effect_id=message_effect_id,
                suggested_post_parameters=suggested_post_parameters,
                reply_parameters=reply_parameters,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
