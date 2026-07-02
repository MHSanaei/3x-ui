from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import UserProfilePhotos
from .base import TelegramMethod


class GetUserProfilePhotos(TelegramMethod[UserProfilePhotos]):
    """
    Use this method to get a list of profile pictures for a user. Returns a :class:`aiogram.types.user_profile_photos.UserProfilePhotos` object.

    Source: https://core.telegram.org/bots/api#getuserprofilephotos
    """

    __returning__ = UserProfilePhotos
    __api_method__ = "getUserProfilePhotos"

    user_id: int
    """Unique identifier of the target user"""
    offset: int | None = None
    """Sequential number of the first photo to be returned. By default, all photos are returned"""
    limit: int | None = None
    """Limits the number of photos to be retrieved. Values between 1-100 are accepted. Defaults to 100"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            offset: int | None = None,
            limit: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, offset=offset, limit=limit, **__pydantic_kwargs)
