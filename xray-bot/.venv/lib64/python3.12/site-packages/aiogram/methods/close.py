from __future__ import annotations

from .base import TelegramMethod


class Close(TelegramMethod[bool]):
    """
    Use this method to close the bot instance before moving it from one local server to another. You need to delete the webhook before calling this method to ensure that the bot isn't launched again after server restart. The method will return error 429 in the first 10 minutes after the bot is launched. Returns :code:`True` on success. Requires no parameters.

    Source: https://core.telegram.org/bots/api#close
    """

    __returning__ = bool
    __api_method__ = "close"
