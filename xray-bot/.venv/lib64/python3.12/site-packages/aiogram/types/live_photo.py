from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class LivePhoto(TelegramObject):
    """
    This object represents a live photo.

    Source: https://core.telegram.org/bots/api#livephoto
    """

    file_id: str
    """Identifier for the video file which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for the video file which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    width: int
    """Video width as defined by the sender"""
    height: int
    """Video height as defined by the sender"""
    duration: int
    """Duration of the video in seconds as defined by the sender"""
    photo: list[PhotoSize] | None = None
    """*Optional*. Available sizes of the corresponding static photo"""
    mime_type: str | None = None
    """*Optional*. MIME type of the file as defined by the sender"""
    file_size: int | None = None
    """*Optional*. File size in bytes. It can be bigger than 2^31 and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a signed 64-bit integer or double-precision float type are safe for storing this value"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            file_id: str,
            file_unique_id: str,
            width: int,
            height: int,
            duration: int,
            photo: list[PhotoSize] | None = None,
            mime_type: str | None = None,
            file_size: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                file_id=file_id,
                file_unique_id=file_unique_id,
                width=width,
                height=height,
                duration=duration,
                photo=photo,
                mime_type=mime_type,
                file_size=file_size,
                **__pydantic_kwargs,
            )
