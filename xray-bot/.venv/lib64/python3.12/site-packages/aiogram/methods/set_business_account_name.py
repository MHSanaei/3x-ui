from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetBusinessAccountName(TelegramMethod[bool]):
    """
    Changes the first and last name of a managed business account. Requires the *can_change_name* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setbusinessaccountname
    """

    __returning__ = bool
    __api_method__ = "setBusinessAccountName"

    business_connection_id: str
    """Unique identifier of the business connection"""
    first_name: str
    """The new value of the first name for the business account; 1-64 characters"""
    last_name: str | None = None
    """The new value of the last name for the business account; 0-64 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            first_name: str,
            last_name: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                first_name=first_name,
                last_name=last_name,
                **__pydantic_kwargs,
            )
