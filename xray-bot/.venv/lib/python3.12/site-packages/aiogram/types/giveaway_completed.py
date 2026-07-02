from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message


class GiveawayCompleted(TelegramObject):
    """
    This object represents a service message about the completion of a giveaway without public winners.

    Source: https://core.telegram.org/bots/api#giveawaycompleted
    """

    winner_count: int
    """Number of winners in the giveaway"""
    unclaimed_prize_count: int | None = None
    """*Optional*. Number of undistributed prizes"""
    giveaway_message: Message | None = None
    """*Optional*. Message with the giveaway that was completed, if it wasn't deleted"""
    is_star_giveaway: bool | None = None
    """*Optional*. :code:`True`, if the giveaway is a Telegram Star giveaway. Otherwise, currently, the giveaway is a Telegram Premium giveaway"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            winner_count: int,
            unclaimed_prize_count: int | None = None,
            giveaway_message: Message | None = None,
            is_star_giveaway: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                winner_count=winner_count,
                unclaimed_prize_count=unclaimed_prize_count,
                giveaway_message=giveaway_message,
                is_star_giveaway=is_star_giveaway,
                **__pydantic_kwargs,
            )
