from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class Dice(TelegramObject):
    """
    This object represents an animated emoji that displays a random value.

    Source: https://core.telegram.org/bots/api#dice
    """

    emoji: str
    """Emoji on which the dice throw animation is based"""
    value: int
    """Value of the dice, 1-6 for '🎲', '🎯' and '🎳' base emoji, 1-5 for '🏀' and '⚽' base emoji, 1-64 for '🎰' base emoji"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, emoji: str, value: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(emoji=emoji, value=value, **__pydantic_kwargs)


class DiceEmoji:
    DICE = "🎲"
    DART = "🎯"
    BASKETBALL = "🏀"
    FOOTBALL = "⚽"
    SLOT_MACHINE = "🎰"
    BOWLING = "🎳"
