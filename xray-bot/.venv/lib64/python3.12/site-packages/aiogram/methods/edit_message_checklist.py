from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InlineKeyboardMarkup, InputChecklist, Message
from .base import TelegramMethod


class EditMessageChecklist(TelegramMethod[Message]):
    """
    Use this method to edit a checklist on behalf of a connected business account. On success, the edited :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#editmessagechecklist
    """

    __returning__ = Message
    __api_method__ = "editMessageChecklist"

    business_connection_id: str
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot in the format :code:`@username`"""
    message_id: int
    """Unique identifier for the target message"""
    checklist: InputChecklist
    """A JSON-serialized object for the new checklist"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for the new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ for the message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            chat_id: ChatIdUnion,
            message_id: int,
            checklist: InputChecklist,
            reply_markup: InlineKeyboardMarkup | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                chat_id=chat_id,
                message_id=message_id,
                checklist=checklist,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
