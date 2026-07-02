from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class UserRating(TelegramObject):
    """
    This object describes the rating of a user based on their Telegram Star spendings.

    Source: https://core.telegram.org/bots/api#userrating
    """

    level: int
    """Current level of the user, indicating their reliability when purchasing digital goods and services. A higher level suggests a more trustworthy customer; a negative level is likely reason for concern"""
    rating: int
    """Numerical value of the user's rating; the higher the rating, the better"""
    current_level_rating: int
    """The rating value required to get the current level"""
    next_level_rating: int | None = None
    """*Optional*. The rating value required to get to the next level; omitted if the maximum level was reached"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            level: int,
            rating: int,
            current_level_rating: int,
            next_level_rating: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                level=level,
                rating=rating,
                current_level_rating=current_level_rating,
                next_level_rating=next_level_rating,
                **__pydantic_kwargs,
            )
