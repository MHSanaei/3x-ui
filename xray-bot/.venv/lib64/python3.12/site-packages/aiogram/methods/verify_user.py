from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class VerifyUser(TelegramMethod[bool]):
    """
    Verifies a user `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ which is represented by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#verifyuser
    """

    __returning__ = bool
    __api_method__ = "verifyUser"

    user_id: int
    """Unique identifier of the target user"""
    custom_description: str | None = None
    """Custom description for the verification; 0-70 characters. Must be empty if the organization isn't allowed to provide a custom verification description"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            custom_description: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id, custom_description=custom_description, **__pydantic_kwargs
            )
