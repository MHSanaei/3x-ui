from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputProfilePhotoUnion
from .base import TelegramMethod


class SetBusinessAccountProfilePhoto(TelegramMethod[bool]):
    """
    Changes the profile photo of a managed business account. Requires the *can_edit_profile_photo* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setbusinessaccountprofilephoto
    """

    __returning__ = bool
    __api_method__ = "setBusinessAccountProfilePhoto"

    business_connection_id: str
    """Unique identifier of the business connection"""
    photo: InputProfilePhotoUnion
    """The new profile photo to set"""
    is_public: bool | None = None
    """Pass :code:`True` to set the public photo, which will be visible even if the main photo is hidden by the business account's privacy settings. An account can have only one public photo"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            photo: InputProfilePhotoUnion,
            is_public: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                photo=photo,
                is_public=is_public,
                **__pydantic_kwargs,
            )
