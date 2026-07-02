from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class RemoveUserVerification(TelegramMethod[bool]):
    """
    Removes verification from a user who is currently verified `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ represented by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#removeuserverification
    """

    __returning__ = bool
    __api_method__ = "removeUserVerification"

    user_id: int
    """Unique identifier of the target user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, user_id: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, **__pydantic_kwargs)
