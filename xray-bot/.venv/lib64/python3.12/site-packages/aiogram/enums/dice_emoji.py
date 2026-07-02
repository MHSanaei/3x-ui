from enum import Enum


class DiceEmoji(str, Enum):
    """
    Emoji on which the dice throw animation is based

    Source: https://core.telegram.org/bots/api#dice
    """

    DICE = "🎲"
    DART = "🎯"
    BASKETBALL = "🏀"
    FOOTBALL = "⚽"
    SLOT_MACHINE = "🎰"
    BOWLING = "🎳"
