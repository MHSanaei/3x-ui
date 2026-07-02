from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .unique_gift import UniqueGift


class UniqueGiftInfo(TelegramObject):
    """
    Describes a service message about a unique gift that was sent or received.

    Source: https://core.telegram.org/bots/api#uniquegiftinfo
    """

    gift: UniqueGift
    """Information about the gift"""
    origin: str
    """Origin of the gift. Currently, either 'upgrade' for gifts upgraded from regular gifts, 'transfer' for gifts transferred from other users or channels, 'resale' for gifts bought from other users, 'gifted_upgrade' for upgrades purchased after the gift was sent, or 'offer' for gifts bought or sold through gift purchase offers"""
    last_resale_currency: str | None = None
    """*Optional*. For gifts bought from other users, the currency in which the payment for the gift was done. Currently, one of 'XTR' for Telegram Stars or 'TON' for toncoins"""
    last_resale_amount: int | None = None
    """*Optional*. For gifts bought from other users, the price paid for the gift in either Telegram Stars or nanotoncoins"""
    owned_gift_id: str | None = None
    """*Optional*. Unique identifier of the received gift for the bot; only present for gifts received on behalf of business accounts"""
    transfer_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that must be paid to transfer the gift; omitted if the bot cannot transfer the gift"""
    next_transfer_date: DateTime | None = None
    """*Optional*. Point in time (Unix timestamp) when the gift can be transferred. If it is in the past, then the gift can be transferred now"""
    last_resale_star_count: int | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For gifts bought from other users, the price paid for the gift

.. deprecated:: API:9.3
   https://core.telegram.org/bots/api-changelog#december-31-2025"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            gift: UniqueGift,
            origin: str,
            last_resale_currency: str | None = None,
            last_resale_amount: int | None = None,
            owned_gift_id: str | None = None,
            transfer_star_count: int | None = None,
            next_transfer_date: DateTime | None = None,
            last_resale_star_count: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                gift=gift,
                origin=origin,
                last_resale_currency=last_resale_currency,
                last_resale_amount=last_resale_amount,
                owned_gift_id=owned_gift_id,
                transfer_star_count=transfer_star_count,
                next_transfer_date=next_transfer_date,
                last_resale_star_count=last_resale_star_count,
                **__pydantic_kwargs,
            )
