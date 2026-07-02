from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetBusinessAccountUsername(TelegramMethod[bool]):
    """
    Changes the username of a managed business account. Requires the *can_change_username* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setbusinessaccountusername
    """

    __returning__ = bool
    __api_method__ = "setBusinessAccountUsername"

    business_connection_id: str
    """Unique identifier of the business connection"""
    username: str | None = None
    """The new value of the username for the business account; 0-32 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            username: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                username=username,
                **__pydantic_kwargs,
            )
