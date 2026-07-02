from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatBoostSourceType
from .chat_boost_source import ChatBoostSource

if TYPE_CHECKING:
    from .user import User


class ChatBoostSourceGiveaway(ChatBoostSource):
    """
    The boost was obtained by the creation of a Telegram Premium or a Telegram Star giveaway. This boosts the chat 4 times for the duration of the corresponding Telegram Premium subscription for Telegram Premium giveaways and *prize_star_count* / 500 times for one year for Telegram Star giveaways.

    Source: https://core.telegram.org/bots/api#chatboostsourcegiveaway
    """

    source: Literal[ChatBoostSourceType.GIVEAWAY] = ChatBoostSourceType.GIVEAWAY
    """Source of the boost, always 'giveaway'"""
    giveaway_message_id: int
    """Identifier of a message in the chat with the giveaway; the message could have been deleted already. May be 0 if the message isn't sent yet"""
    user: User | None = None
    """*Optional*. User that won the prize in the giveaway if any; for Telegram Premium giveaways only"""
    prize_star_count: int | None = None
    """*Optional*. The number of Telegram Stars to be split between giveaway winners; for Telegram Star giveaways only"""
    is_unclaimed: bool | None = None
    """*Optional*. :code:`True`, if the giveaway was completed, but there was no user to win the prize"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[ChatBoostSourceType.GIVEAWAY] = ChatBoostSourceType.GIVEAWAY,
            giveaway_message_id: int,
            user: User | None = None,
            prize_star_count: int | None = None,
            is_unclaimed: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                source=source,
                giveaway_message_id=giveaway_message_id,
                user=user,
                prize_star_count=prize_star_count,
                is_unclaimed=is_unclaimed,
                **__pydantic_kwargs,
            )
