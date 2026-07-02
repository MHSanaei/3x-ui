from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class GiveawayCreated(TelegramObject):
    """
    This object represents a service message about the creation of a scheduled giveaway.

    Source: https://core.telegram.org/bots/api#giveawaycreated
    """

    prize_star_count: int | None = None
    """*Optional*. The number of Telegram Stars to be split between giveaway winners; for Telegram Star giveaways only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, prize_star_count: int | None = None, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(prize_star_count=prize_star_count, **__pydantic_kwargs)
