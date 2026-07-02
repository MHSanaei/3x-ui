from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import OwnedGiftType

from .custom import DateTime
from .owned_gift import OwnedGift

if TYPE_CHECKING:
    from .unique_gift import UniqueGift
    from .user import User


class OwnedGiftUnique(OwnedGift):
    """
    Describes a unique gift received and owned by a user or a chat.

    Source: https://core.telegram.org/bots/api#ownedgiftunique
    """

    type: Literal[OwnedGiftType.UNIQUE] = OwnedGiftType.UNIQUE
    """Type of the gift, always 'unique'"""
    gift: UniqueGift
    """Information about the unique gift"""
    send_date: int
    """Date the gift was sent in Unix time"""
    owned_gift_id: str | None = None
    """*Optional*. Unique identifier of the received gift for the bot; for gifts received on behalf of business accounts only"""
    sender_user: User | None = None
    """*Optional*. Sender of the gift if it is a known user"""
    is_saved: bool | None = None
    """*Optional*. :code:`True`, if the gift is displayed on the account's profile page; for gifts received on behalf of business accounts only"""
    can_be_transferred: bool | None = None
    """*Optional*. :code:`True`, if the gift can be transferred to another owner; for gifts received on behalf of business accounts only"""
    transfer_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that must be paid to transfer the gift; omitted if the bot cannot transfer the gift"""
    next_transfer_date: DateTime | None = None
    """*Optional*. Point in time (Unix timestamp) when the gift can be transferred. If it is in the past, then the gift can be transferred now"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[OwnedGiftType.UNIQUE] = OwnedGiftType.UNIQUE,
            gift: UniqueGift,
            send_date: int,
            owned_gift_id: str | None = None,
            sender_user: User | None = None,
            is_saved: bool | None = None,
            can_be_transferred: bool | None = None,
            transfer_star_count: int | None = None,
            next_transfer_date: DateTime | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                gift=gift,
                send_date=send_date,
                owned_gift_id=owned_gift_id,
                sender_user=sender_user,
                is_saved=is_saved,
                can_be_transferred=can_be_transferred,
                transfer_star_count=transfer_star_count,
                next_transfer_date=next_transfer_date,
                **__pydantic_kwargs,
            )
