from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .gift import Gift
    from .message_entity import MessageEntity


class GiftInfo(TelegramObject):
    """
    Describes a service message about a regular gift that was sent or received.

    Source: https://core.telegram.org/bots/api#giftinfo
    """

    gift: Gift
    """Information about the gift"""
    owned_gift_id: str | None = None
    """*Optional*. Unique identifier of the received gift for the bot; only present for gifts received on behalf of business accounts"""
    convert_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that can be claimed by the receiver by converting the gift; omitted if conversion to Telegram Stars is impossible"""
    prepaid_upgrade_star_count: int | None = None
    """*Optional*. Number of Telegram Stars that were prepaid for the ability to upgrade the gift"""
    is_upgrade_separate: bool | None = None
    """*Optional*. :code:`True`, if the gift's upgrade was purchased after the gift was sent"""
    can_be_upgraded: bool | None = None
    """*Optional*. :code:`True`, if the gift can be upgraded to a unique gift"""
    text: str | None = None
    """*Optional*. Text of the message that was added to the gift"""
    entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the text"""
    is_private: bool | None = None
    """*Optional*. :code:`True`, if the sender and gift text are shown only to the gift receiver; otherwise, everyone will be able to see them"""
    unique_gift_number: int | None = None
    """*Optional*. Unique number reserved for this gift when upgraded. See the *number* field in :class:`aiogram.types.unique_gift.UniqueGift`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            gift: Gift,
            owned_gift_id: str | None = None,
            convert_star_count: int | None = None,
            prepaid_upgrade_star_count: int | None = None,
            is_upgrade_separate: bool | None = None,
            can_be_upgraded: bool | None = None,
            text: str | None = None,
            entities: list[MessageEntity] | None = None,
            is_private: bool | None = None,
            unique_gift_number: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                gift=gift,
                owned_gift_id=owned_gift_id,
                convert_star_count=convert_star_count,
                prepaid_upgrade_star_count=prepaid_upgrade_star_count,
                is_upgrade_separate=is_upgrade_separate,
                can_be_upgraded=can_be_upgraded,
                text=text,
                entities=entities,
                is_private=is_private,
                unique_gift_number=unique_gift_number,
                **__pydantic_kwargs,
            )
