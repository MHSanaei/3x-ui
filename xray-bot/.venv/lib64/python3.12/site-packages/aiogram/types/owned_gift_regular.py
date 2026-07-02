from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import OwnedGiftType

from .owned_gift import OwnedGift

if TYPE_CHECKING:
    from .gift import Gift
    from .message_entity import MessageEntity
    from .user import User


class OwnedGiftRegular(OwnedGift):
    """
    Describes a regular gift owned by a user or a chat.

    Source: https://core.telegram.org/bots/api#ownedgiftregular
    """

    type: Literal[OwnedGiftType.REGULAR] = OwnedGiftType.REGULAR
    """Type of the gift, always 'regular'"""
    gift: Gift
    """Information about the regular gift"""
    send_date: int
    """Date the gift was sent in Unix time"""
    owned_gift_id: str | None = None
    """*Optional*. Unique identifier of the gift for the bot; for gifts received on behalf of business accounts only"""
    sender_user: User | None = None
    """*Optional*. Sender of the gift if it is a known user"""
    text: str | None = None
    """*Optional*. Text of the message that was added to the gift"""
    entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the text"""
    is_private: bool | None = None
    """*Optional*. :code:`True`, if the sender and gift text are shown only to the gift receiver; otherwise, everyone will be able to see them"""
    is_saved: bool | None = None
    """*Optional*. :code:`True`, if the gift is displayed on the account's profile page; for gifts received on behalf of business accounts only"""
    can_be_upgraded: bool | None = None
    """*Optional*. :code:`True`, if the gift can be upgraded to a unique gift; for gifts received on behalf of business accounts only"""
    was_refunded: bool | None = None
    """*Optional*. :code:`True`, if the gift was refunded and isn't available anymore"""
    convert_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that can be claimed by the receiver instead of the gift; omitted if the gift cannot be converted to Telegram Stars; for gifts received on behalf of business accounts only"""
    prepaid_upgrade_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that were paid for the ability to upgrade the gift"""
    is_upgrade_separate: bool | None = None
    """*Optional*. :code:`True`, if the gift's upgrade was purchased after the gift was sent; for gifts received on behalf of business accounts only"""
    unique_gift_number: int | None = None
    """*Optional*. Unique number reserved for this gift when upgraded. See the *number* field in :class:`aiogram.types.unique_gift.UniqueGift`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[OwnedGiftType.REGULAR] = OwnedGiftType.REGULAR,
            gift: Gift,
            send_date: int,
            owned_gift_id: str | None = None,
            sender_user: User | None = None,
            text: str | None = None,
            entities: list[MessageEntity] | None = None,
            is_private: bool | None = None,
            is_saved: bool | None = None,
            can_be_upgraded: bool | None = None,
            was_refunded: bool | None = None,
            convert_star_count: int | None = None,
            prepaid_upgrade_star_count: int | None = None,
            is_upgrade_separate: bool | None = None,
            unique_gift_number: int | None = None,
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
                text=text,
                entities=entities,
                is_private=is_private,
                is_saved=is_saved,
                can_be_upgraded=can_be_upgraded,
                was_refunded=was_refunded,
                convert_star_count=convert_star_count,
                prepaid_upgrade_star_count=prepaid_upgrade_star_count,
                is_upgrade_separate=is_upgrade_separate,
                unique_gift_number=unique_gift_number,
                **__pydantic_kwargs,
            )
