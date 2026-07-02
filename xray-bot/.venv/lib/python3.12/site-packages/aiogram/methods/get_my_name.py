from typing import TYPE_CHECKING, Any

from ..types import BotName
from .base import TelegramMethod


class GetMyName(TelegramMethod[BotName]):
    """
    Use this method to get the current bot name for the given user language. Returns :class:`aiogram.types.bot_name.BotName` on success.

    Source: https://core.telegram.org/bots/api#getmyname
    """

    __returning__ = BotName
    __api_method__ = "getMyName"

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
