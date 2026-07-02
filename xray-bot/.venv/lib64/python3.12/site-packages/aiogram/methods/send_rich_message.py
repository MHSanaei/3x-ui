from typing import TYPE_CHECKING, Any

from ..types import (
    ChatIdUnion,
    InputRichMessage,
    Message,
    ReplyMarkupUnion,
    ReplyParameters,
    SuggestedPostParameters,
)
from .base import TelegramMethod


class SendRichMessage(TelegramMethod[Message]):
    """
    Use this method to send rich messages. If the message contains a block with a media element, then the bot must have the right to send the media to the chat. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendrichmessage
    """

    __returning__ = Message
    __api_method__ = "sendRichMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    rich_message: InputRichMessage
    """The message to be sent"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    direct_messages_topic_id: int | None = None
    """Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat"""
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
            rich_message: InputRichMessage,
            business_connection_id: str | None = None,
            message_thread_id: int | None = None,
            direct_messages_topic_id: int | None = None,
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
                rich_message=rich_message,
                business_connection_id=business_connection_id,
                message_thread_id=message_thread_id,
                direct_messages_topic_id=direct_messages_topic_id,
                disable_notification=disable_notification,
                protect_content=protect_content,
                allow_paid_broadcast=allow_paid_broadcast,
                message_effect_id=message_effect_id,
                suggested_post_parameters=suggested_post_parameters,
                reply_parameters=reply_parameters,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
