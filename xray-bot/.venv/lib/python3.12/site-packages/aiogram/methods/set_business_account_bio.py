from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetBusinessAccountBio(TelegramMethod[bool]):
    """
    Changes the bio of a managed business account. Requires the *can_change_bio* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setbusinessaccountbio
    """

    __returning__ = bool
    __api_method__ = "setBusinessAccountBio"

    business_connection_id: str
    """Unique identifier of the business connection"""
    bio: str | None = None
    """The new value of the bio for the business account; 0-140 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            bio: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id, bio=bio, **__pydantic_kwargs
            )
