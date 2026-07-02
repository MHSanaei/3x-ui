from __future__ import annotations

from typing import TYPE_CHECKING, Any

from aiogram.types.base import TelegramObject

if TYPE_CHECKING:
    from .update import Update


class ErrorEvent(TelegramObject):
    """
    Internal event, should be used to receive errors while processing Updates from Telegram

    Source: https://core.telegram.org/bots/api#error-event
    """

    update: Update
    """Received update"""
    exception: Exception
    """Exception"""

    if TYPE_CHECKING:

        def __init__(
            __pydantic_self__, *, update: Update, exception: Exception, **__pydantic_kwargs: Any
        ) -> None:
            super().__init__(update=update, exception=exception, **__pydantic_kwargs)
