from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InlineKeyboardMarkup, InputMediaUnion, Message
from .base import TelegramMethod


class EditMessageMedia(TelegramMethod[Message | bool]):
    """
    Use this method to edit animation, audio, document, live photo, photo, or video messages, or to replace a text or a rich message with a media. If a message is part of a message album, then it can be edited only to an audio for audio albums, only to a document for document albums and to a photo, a live photo, or a video otherwise. When an inline message is edited, a new file can't be uploaded; use a previously uploaded file via its file_id or specify a URL. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

    Source: https://core.telegram.org/bots/api#editmessagemedia
    """

    __returning__ = Message | bool
    __api_method__ = "editMessageMedia"

    media: InputMediaUnion
    """A JSON-serialized object for a new media content of the message"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message to be edited was sent"""
    chat_id: ChatIdUnion | None = None
    """Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_id: int | None = None
    """Required if *inline_message_id* is not specified. Identifier of the message to edit"""
    inline_message_id: str | None = None
    """Required if *chat_id* and *message_id* are not specified. Identifier of the inline message"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            media: InputMediaUnion,
            business_connection_id: str | None = None,
            chat_id: ChatIdUnion | None = None,
            message_id: int | None = None,
            inline_message_id: str | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                media=media,
                business_connection_id=business_connection_id,
                chat_id=chat_id,
                message_id=message_id,
                inline_message_id=inline_message_id,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
