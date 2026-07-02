from __future__ import annotations

from ..types import StarAmount
from .base import TelegramMethod


class GetMyStarBalance(TelegramMethod[StarAmount]):
    """
    A method to get the current Telegram Stars balance of the bot. Requires no parameters. On success, returns a :class:`aiogram.types.star_amount.StarAmount` object.

    Source: https://core.telegram.org/bots/api#getmystarbalance
    """

    __returning__ = StarAmount
    __api_method__ = "getMyStarBalance"
