from __future__ import annotations

from .base import TelegramObject


class OwnedGift(TelegramObject):
    """
    This object describes a gift received and owned by a user or a chat. Currently, it can be one of

     - :class:`aiogram.types.owned_gift_regular.OwnedGiftRegular`
     - :class:`aiogram.types.owned_gift_unique.OwnedGiftUnique`

    Source: https://core.telegram.org/bots/api#ownedgift
    """
