from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class ResponseParameters(TelegramObject):
    """
    Describes why a request was unsuccessful.

    Source: https://core.telegram.org/bots/api#responseparameters
    """

    migrate_to_chat_id: int | None = None
    """*Optional*. The group has been migrated to a supergroup with the specified identifier. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a signed 64-bit integer or double-precision float type are safe for storing this identifier"""
    retry_after: int | None = None
    """*Optional*. In case of exceeding flood control, the number of seconds left to wait before the request can be repeated"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            migrate_to_chat_id: int | None = None,
            retry_after: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                migrate_to_chat_id=migrate_to_chat_id, retry_after=retry_after, **__pydantic_kwargs
            )
