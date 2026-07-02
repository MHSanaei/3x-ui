from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .custom import DateTime


class Giveaway(TelegramObject):
    """
    This object represents a message about a scheduled giveaway.

    Source: https://core.telegram.org/bots/api#giveaway
    """

    chats: list[Chat]
    """The list of chats which the user must join to participate in the giveaway"""
    winners_selection_date: DateTime
    """Point in time (Unix timestamp) when winners of the giveaway will be selected"""
    winner_count: int
    """The number of users which are supposed to be selected as winners of the giveaway"""
    only_new_members: bool | None = None
    """*Optional*. :code:`True`, if only users who join the chats after the giveaway started should be eligible to win"""
    has_public_winners: bool | None = None
    """*Optional*. :code:`True`, if the list of giveaway winners will be visible to everyone"""
    prize_description: str | None = None
    """*Optional*. Description of additional giveaway prize"""
    country_codes: list[str] | None = None
    """*Optional*. A list of two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which eligible users for the giveaway must come. If empty, then all users can participate in the giveaway. Users with a phone number that was bought on Fragment can always participate in giveaways"""
    prize_star_count: int | None = None
    """*Optional*. The number of Telegram Stars to be split between giveaway winners; for Telegram Star giveaways only"""
    premium_subscription_month_count: int | None = None
    """*Optional*. The number of months the Telegram Premium subscription won from the giveaway will be active for; for Telegram Premium giveaways only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chats: list[Chat],
            winners_selection_date: DateTime,
            winner_count: int,
            only_new_members: bool | None = None,
            has_public_winners: bool | None = None,
            prize_description: str | None = None,
            country_codes: list[str] | None = None,
            prize_star_count: int | None = None,
            premium_subscription_month_count: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chats=chats,
                winners_selection_date=winners_selection_date,
                winner_count=winner_count,
                only_new_members=only_new_members,
                has_public_winners=has_public_winners,
                prize_description=prize_description,
                country_codes=country_codes,
                prize_star_count=prize_star_count,
                premium_subscription_month_count=premium_subscription_month_count,
                **__pydantic_kwargs,
            )
