from __future__ import annotations

from ..types.gifts import Gifts
from .base import TelegramMethod


class GetAvailableGifts(TelegramMethod[Gifts]):
    """
    Returns the list of gifts that can be sent by the bot to users and channel chats. Requires no parameters. Returns a :class:`aiogram.types.gifts.Gifts` object.

    Source: https://core.telegram.org/bots/api#getavailablegifts
    """

    __returning__ = Gifts
    __api_method__ = "getAvailableGifts"
