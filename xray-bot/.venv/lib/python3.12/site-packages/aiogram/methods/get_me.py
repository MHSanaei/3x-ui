from __future__ import annotations

from ..types import User
from .base import TelegramMethod


class GetMe(TelegramMethod[User]):
    """
    A simple method for testing your bot's authentication token. Requires no parameters. Returns basic information about the bot in form of a :class:`aiogram.types.user.User` object.

    Source: https://core.telegram.org/bots/api#getme
    """

    __returning__ = User
    __api_method__ = "getMe"
