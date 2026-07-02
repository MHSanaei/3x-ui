from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetMyShortDescription(TelegramMethod[bool]):
    """
    Use this method to change the bot's short description, which is shown on the bot's profile page and is sent together with the link when users share the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmyshortdescription
    """

    __returning__ = bool
    __api_method__ = "setMyShortDescription"

    short_description: str | None = None
    """New short description for the bot; 0-120 characters. Pass an empty string to remove the dedicated short description for the given language"""
    language_code: str | None = None
    """A two-letter ISO 639-1 language code. If empty, the short description will be applied to all users for whose language there is no dedicated short description"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            short_description: str | None = None,
            language_code: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                short_description=short_description,
                language_code=language_code,
                **__pydantic_kwargs,
            )
