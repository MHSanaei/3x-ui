from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetMyName(TelegramMethod[bool]):
    """
    Use this method to change the bot's name. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmyname
    """

    __returning__ = bool
    __api_method__ = "setMyName"

    name: str | None = None
    """New bot name; 0-64 characters. Pass an empty string to remove the dedicated name for the given language"""
    language_code: str | None = None
    """A two-letter ISO 639-1 language code. If empty, the name will be shown to all users for whose language there is no dedicated name"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str | None = None,
            language_code: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, language_code=language_code, **__pydantic_kwargs)
