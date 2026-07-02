from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from ..types import (
    ChatIdUnion,
    DateTimeUnion,
    InputFile,
    InputFileUnion,
    Message,
    MessageEntity,
    ReplyMarkupUnion,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class SendVideo(TelegramMethod[Message]):
    """
    Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

    Source: https://core.telegram.org/bots/api#sendvideo
    """

    __returning__ = Message
    __api_method__ = "sendVideo"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    video: InputFileUnion
    """Video to send. Pass a file_id as String to send a video that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a video from the Internet, or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
    duration: int | None = None
    """Duration of sent video in seconds"""
    width: int | None = None
    """Video width"""
    height: int | None = None
    """Video height"""
    thumbnail: InputFile | None = None
    """Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`"""
    cover: InputFileUnion | None = None
    """Cover for the video in the message. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`"""
    start_timestamp: DateTimeUnion | None = None
    """Start timestamp for the video in the message"""
    caption: str | None = None
    """Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | Default | None = Default("show_caption_above_media")
    """Pass :code:`True`, if the caption must be shown above the message media"""
    has_spoiler: bool | None = None
    """Pass :code:`True` if the video needs to be covered with a spoiler animation"""
    supports_streaming: bool | None = None
    """Pass :code:`True` if the uploaded video is suitable for streaming"""
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
            video: InputFileUnion,
            business_connection_id: str | None = None,
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
            duration: int | None = None,
            width: int | None = None,
            height: int | None = None,
            thumbnail: InputFile | None = None,
            cover: InputFileUnion | None = None,
            start_timestamp: DateTimeUnion | None = None,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
            has_spoiler: bool | None = None,
            supports_streaming: bool | None = None,
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
                video=video,
                business_connection_id=business_connection_id,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                duration=duration,
                width=width,
                height=height,
                thumbnail=thumbnail,
                cover=cover,
                start_timestamp=start_timestamp,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                has_spoiler=has_spoiler,
                supports_streaming=supports_streaming,
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
