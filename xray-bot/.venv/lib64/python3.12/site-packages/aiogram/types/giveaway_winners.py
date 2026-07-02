from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .custom import DateTime
    from .user import User


class GiveawayWinners(TelegramObject):
    """
    This object represents a message about the completion of a giveaway with public winners.

    Source: https://core.telegram.org/bots/api#giveawaywinners
    """

    chat: Chat
    """The chat that created the giveaway"""
    giveaway_message_id: int
    """Identifier of the message with the giveaway in the chat"""
    winners_selection_date: DateTime
    """Point in time (Unix timestamp) when winners of the giveaway were selected"""
    winner_count: int
    """Total number of winners in the giveaway"""
    winners: list[User]
    """List of up to 100 winners of the giveaway"""
    additional_chat_count: int | None = None
    """*Optional*. The number of other chats the user had to join in order to be eligible for the giveaway"""
    prize_star_count: int | None = None
    """*Optional*. The number of Telegram Stars that were split between giveaway winners; for Telegram Star giveaways only"""
    premium_subscription_month_count: int | None = None
    """*Optional*. The number of months the Telegram Premium subscription won from the giveaway will be active for; for Telegram Premium giveaways only"""
    unclaimed_prize_count: int | None = None
    """*Optional*. Number of undistributed prizes"""
    only_new_members: bool | None = None
    """*Optional*. :code:`True`, if only users who had joined the chats after the giveaway started were eligible to win"""
    was_refunded: bool | None = None
    """*Optional*. :code:`True`, if the giveaway was canceled because the payment for it was refunded"""
    prize_description: str | None = None
    """*Optional*. Description of additional giveaway prize"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat: Chat,
            giveaway_message_id: int,
            winners_selection_date: DateTime,
            winner_count: int,
            winners: list[User],
            additional_chat_count: int | None = None,
            prize_star_count: int | None = None,
            premium_subscription_month_count: int | None = None,
            unclaimed_prize_count: int | None = None,
            only_new_members: bool | None = None,
            was_refunded: bool | None = None,
            prize_description: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat=chat,
                giveaway_message_id=giveaway_message_id,
                winners_selection_date=winners_selection_date,
                winner_count=winner_count,
                winners=winners,
                additional_chat_count=additional_chat_count,
                prize_star_count=prize_star_count,
                premium_subscription_month_count=premium_subscription_month_count,
                unclaimed_prize_count=unclaimed_prize_count,
                only_new_members=only_new_members,
                was_refunded=was_refunded,
                prize_description=prize_description,
                **__pydantic_kwargs,
            )
