from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class GameHighScore(TelegramObject):
    """
    This object represents one row of the high scores table for a game.
    And that's about all we've got for now.

    If you've got any questions, please check out our `https://core.telegram.org/bots/faq <https://core.telegram.org/bots/faq>`_ **Bot FAQ »**

    Source: https://core.telegram.org/bots/api#gamehighscore
    """

    position: int
    """Position in high score table for the game"""
    user: User
    """User"""
    score: int
    """Score"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, position: int, user: User, score: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(position=position, user=user, score=score, **__pydantic_kwargs)
