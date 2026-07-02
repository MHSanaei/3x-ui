from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .sticker import Sticker


class BusinessIntro(TelegramObject):
    """
    Contains information about the start page settings of a Telegram Business account.

    Source: https://core.telegram.org/bots/api#businessintro
    """

    title: str | None = None
    """*Optional*. Title text of the business intro"""
    message: str | None = None
    """*Optional*. Message text of the business intro"""
    sticker: Sticker | None = None
    """*Optional*. Sticker of the business intro"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str | None = None,
            message: str | None = None,
            sticker: Sticker | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(title=title, message=message, sticker=sticker, **__pydantic_kwargs)
