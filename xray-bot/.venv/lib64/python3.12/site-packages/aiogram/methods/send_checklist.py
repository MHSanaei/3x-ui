from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InlineKeyboardMarkup, InputChecklist, Message, ReplyParameters
from .base import TelegramMethod


class SendChecklist(TelegramMethod[Message]):
    """
    Use this method to send a checklist on behalf of a connected business account. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendchecklist
    """

    __returning__ = Message
    __api_method__ = "sendChecklist"

    business_connection_id: str
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot in the format :code:`@username`"""
    checklist: InputChecklist
    """A JSON-serialized object for the checklist to send"""
    disable_notification: bool | None = None
    """Sends the message silently. Users will receive a notification with no sound"""
    protect_content: bool | None = None
    """Protects the contents of the sent message from forwarding and saving"""
    message_effect_id: str | None = None
    """Unique identifier of the message effect to be added to the message"""
    reply_parameters: ReplyParameters | None = None
    """A JSON-serialized object for description of the message to reply to"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            chat_id: ChatIdUnion,
            checklist: InputChecklist,
            disable_notification: bool | None = None,
            protect_content: bool | None = None,
            message_effect_id: str | None = None,
            reply_parameters: ReplyParameters | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                chat_id=chat_id,
                checklist=checklist,
                disable_notification=disable_notification,
                protect_content=protect_content,
                message_effect_id=message_effect_id,
                reply_parameters=reply_parameters,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
