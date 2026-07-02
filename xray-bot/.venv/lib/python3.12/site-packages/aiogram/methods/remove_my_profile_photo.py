from __future__ import annotations

from .base import TelegramMethod


class RemoveMyProfilePhoto(TelegramMethod[bool]):
    """
    Removes the profile photo of the bot. Requires no parameters. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#removemyprofilephoto
    """

    __returning__ = bool
    __api_method__ = "removeMyProfilePhoto"
