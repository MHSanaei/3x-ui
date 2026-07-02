from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..client.default import Default
from ..types import ChatIdUnion, DateTimeUnion, Message, SuggestedPostParameters
from .base import TelegramMethod


class ForwardMessage(TelegramMethod[Message]):
    """
    Use this method to forward messages of any kind. Service messages and messages with protected content can't be forwarded. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#forwardmessage
    """

    __returning__ = Message
    __api_method__ = "forwardMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    from_chat_id: ChatIdUnion
    """Unique identifier for the chat where the original message was sent (or username of the target bot, supergroup or channel in the format :code:`@username`)"""
    message_id: int
    """Message identifier in the chat specified in *from_chat_id*"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be forwarded; required if the message is forwarded to a direct messages chat"""
    video_start_timestamp: DateTimeUnion | None = None
    """New start timestamp for the forwarded video in the message"""
    disable_notification: bool | None = None
    """Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound"""
    protect_content: bool | Default | None = Default("protect_content")
    """Protects the contents of the forwarded message from forwarding and saving"""
    message_effect_id: str | None = None
    """Unique identifier of the message effect to be added to the message; only available when forwarding to private chats"""
    suggested_post_parameters: SuggestedPostParameters | None = None
    """A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only"""

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
            disable_notification: bool | None = None,
            protect_content: bool | Default | None = Default("protect_content"),
            message_effect_id: str | None = None,
            suggested_post_parameters: SuggestedPostParameters | None = None,
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
                disable_notification=disable_notification,
                protect_content=protect_content,
                message_effect_id=message_effect_id,
                suggested_post_parameters=suggested_post_parameters,
                **__pydantic_kwargs,
            )
