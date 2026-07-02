from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import GameHighScore
from .base import TelegramMethod


class GetGameHighScores(TelegramMethod[list[GameHighScore]]):
    """
    Use this method to get data for high score tables. Will return the score of the specified user and several of their neighbors in a game. Returns an Array of :class:`aiogram.types.game_high_score.GameHighScore` objects.

     This method will currently return scores for the target user, plus two of their closest neighbors on each side. Will also return the top three users if the user and their neighbors are not among them. Please note that this behavior is subject to change.

    Source: https://core.telegram.org/bots/api#getgamehighscores
    """

    __returning__ = list[GameHighScore]
    __api_method__ = "getGameHighScores"

    user_id: int
    """Target user id"""
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
                chat_id=chat_id,
                message_id=message_id,
                inline_message_id=inline_message_id,
                **__pydantic_kwargs,
            )
