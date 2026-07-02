from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InlineKeyboardMarkup, Poll
from .base import TelegramMethod


class StopPoll(TelegramMethod[Poll]):
    """
    Use this method to stop a poll which was sent by the bot. On success, the stopped :class:`aiogram.types.poll.Poll` is returned.

    Source: https://core.telegram.org/bots/api#stoppoll
    """

    __returning__ = Poll
    __api_method__ = "stopPoll"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_id: int
    """Identifier of the original message with the poll"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message to be edited was sent"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for a new message `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_id: int,
            business_connection_id: str | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                message_id=message_id,
                business_connection_id=business_connection_id,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
