from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetMyDescription(TelegramMethod[bool]):
    """
    Use this method to change the bot's description, which is shown in the chat with the bot if the chat is empty. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmydescription
    """

    __returning__ = bool
    __api_method__ = "setMyDescription"

    description: str | None = None
    """New bot description; 0-512 characters. Pass an empty string to remove the dedicated description for the given language"""
    language_code: str | None = None
    """A two-letter ISO 639-1 language code. If empty, the description will be applied to all users for whose language there is no dedicated description"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            description: str | None = None,
            language_code: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                description=description, language_code=language_code, **__pydantic_kwargs
            )
