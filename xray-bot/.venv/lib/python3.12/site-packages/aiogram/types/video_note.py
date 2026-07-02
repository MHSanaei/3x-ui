from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class VideoNote(TelegramObject):
    """
    This object represents a `video message <https://telegram.org/blog/video-messages-and-telescope>`_ (available in Telegram apps as of `v.4.0 <https://telegram.org/blog/video-messages-and-telescope>`_).

    Source: https://core.telegram.org/bots/api#videonote
    """

    file_id: str
    """Identifier for this file, which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for this file, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    length: int
    """Video width and height (diameter of the video message) as defined by the sender"""
    duration: int
    """Duration of the video in seconds as defined by the sender"""
    thumbnail: PhotoSize | None = None
    """*Optional*. Video thumbnail"""
    file_size: int | None = None
    """*Optional*. File size in bytes"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            file_id: str,
            file_unique_id: str,
            length: int,
            duration: int,
            thumbnail: PhotoSize | None = None,
            file_size: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                file_id=file_id,
                file_unique_id=file_unique_id,
                length=length,
                duration=duration,
                thumbnail=thumbnail,
                file_size=file_size,
                **__pydantic_kwargs,
            )
