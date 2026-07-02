from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class SendChatAction(TelegramMethod[bool]):
    """
    Use this method when you need to tell the user that something is happening on the bot's side. The status is set for 5 seconds or less (when a message arrives from your bot, Telegram clients clear its typing status). Returns :code:`True` on success.

     Example: The `ImageBot <https://t.me/imagebot>`_ needs some time to process a request and upload the image. Instead of sending a text message along the lines of 'Retrieving image, please wait…', the bot may use :class:`aiogram.methods.send_chat_action.SendChatAction` with *action* = *upload_photo*. The user will see a 'sending photo' status for the bot.

    We only recommend using this method when a response from the bot will take a **noticeable** amount of time to arrive.

    Source: https://core.telegram.org/bots/api#sendchataction
    """

    __returning__ = bool
    __api_method__ = "sendChatAction"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot or supergroup in the format :code:`@username`. Channel chats and channel direct messages chats aren't supported"""
    action: str
    """Type of action to broadcast. Choose one, depending on what the user is about to receive: *typing* for `text messages <https://core.telegram.org/bots/api#sendmessage>`_, *upload_photo* for `photos <https://core.telegram.org/bots/api#sendphoto>`_, *record_video* or *upload_video* for `videos <https://core.telegram.org/bots/api#sendvideo>`_, *record_voice* or *upload_voice* for `voice notes <https://core.telegram.org/bots/api#sendvoice>`_, *upload_document* for `general files <https://core.telegram.org/bots/api#senddocument>`_, *choose_sticker* for `stickers <https://core.telegram.org/bots/api#sendsticker>`_, *find_location* for `location data <https://core.telegram.org/bots/api#sendlocation>`_, *record_video_note* or *upload_video_note* for `video notes <https://core.telegram.org/bots/api#sendvideonote>`_"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the action will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread or topic of a forum; for supergroups and private chats of bots with forum topic mode enabled only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            action: str,
            business_connection_id: str | None = None,
            message_thread_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                action=action,
                business_connection_id=business_connection_id,
                message_thread_id=message_thread_id,
                **__pydantic_kwargs,
            )
