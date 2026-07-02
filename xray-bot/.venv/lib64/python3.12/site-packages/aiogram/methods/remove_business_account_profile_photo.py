from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class RemoveBusinessAccountProfilePhoto(TelegramMethod[bool]):
    """
    Removes the current profile photo of a managed business account. Requires the *can_edit_profile_photo* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#removebusinessaccountprofilephoto
    """

    __returning__ = bool
    __api_method__ = "removeBusinessAccountProfilePhoto"

    business_connection_id: str
    """Unique identifier of the business connection"""
    is_public: bool | None = None
    """Pass :code:`True` to remove the public photo, which is visible even if the main photo is hidden by the business account's privacy settings. After the main photo is removed, the previous profile photo (if present) becomes the main photo"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            is_public: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                is_public=is_public,
                **__pydantic_kwargs,
            )
