from enum import Enum


class PollType(str, Enum):
    """
    This object represents poll type

    Source: https://core.telegram.org/bots/api#poll
    """

    REGULAR = "regular"
    QUIZ = "quiz"
