from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class UserProfilePhotos(TelegramObject):
    """
    This object represent a user's profile pictures.

    Source: https://core.telegram.org/bots/api#userprofilephotos
    """

    total_count: int
    """Total number of profile pictures the target user has"""
    photos: list[list[PhotoSize]]
    """Requested profile pictures (in up to 4 sizes each)"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            total_count: int,
            photos: list[list[PhotoSize]],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(total_count=total_count, photos=photos, **__pydantic_kwargs)
