from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import Message
from .base import TelegramMethod


class SetGameScore(TelegramMethod[Message | bool]):
    """
    Use this method to set the score of the specified user in a game message. On success, if the message is not an inline message, the :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Returns an error, if the new score is not greater than the user's current score in the chat and *force* is :code:`False`.

    Source: https://core.telegram.org/bots/api#setgamescore
    """

    __returning__ = Message | bool
    __api_method__ = "setGameScore"

    user_id: int
    """User identifier"""
    score: int
    """New score, must be non-negative"""
    force: bool | None = None
    """Pass :code:`True` if the high score is allowed to decrease. This can be useful when fixing mistakes or banning cheaters"""
    disable_edit_message: bool | None = None
    """Pass :code:`True` if the game message should not be automatically edited to include the current scoreboard"""
    chat_id: int | None = None
    """Required if *inline_message_id* is not specified. Unique identifier for the target chat"""
    message_id: int | None = None
    """Required if *inline_message_id* is not specified. Identifier of the sent message"""
    inline_message_id: str | None = None
    """Required if *chat_id* and *message_id* are not specified. Identifier of the inline message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            score: int,
            force: bool | None = None,
            disable_edit_message: bool | None = None,
            chat_id: int | None = None,
            message_id: int | None = None,
            inline_message_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                score=score,
                force=force,
                disable_edit_message=disable_edit_message,
                chat_id=chat_id,
                message_id=message_id,
                inline_message_id=inline_message_id,
                **__pydantic_kwargs,
            )
