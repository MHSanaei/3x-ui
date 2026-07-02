from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import BotDescription
from .base import TelegramMethod


class GetMyDescription(TelegramMethod[BotDescription]):
    """
    Use this method to get the current bot description for the given user language. Returns :class:`aiogram.types.bot_description.BotDescription` on success.

    Source: https://core.telegram.org/bots/api#getmydescription
    """

    __returning__ = BotDescription
    __api_method__ = "getMyDescription"

    language_code: str | None = None
    """A two-letter ISO 639-1 language code or an empty string"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, language_code: str | None = None, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(language_code=language_code, **__pydantic_kwargs)
