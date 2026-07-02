from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class Document(TelegramObject):
    """
    This object represents a general file (as opposed to `photos <https://core.telegram.org/bots/api#photosize>`_, `voice messages <https://core.telegram.org/bots/api#voice>`_ and `audio files <https://core.telegram.org/bots/api#audio>`_).

    Source: https://core.telegram.org/bots/api#document
    """

    file_id: str
    """Identifier for this file, which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for this file, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    thumbnail: PhotoSize | None = None
    """*Optional*. Document thumbnail as defined by the sender"""
    file_name: str | None = None
    """*Optional*. Original filename as defined by the sender"""
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
            thumbnail: PhotoSize | None = None,
            file_name: str | None = None,
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
                thumbnail=thumbnail,
                file_name=file_name,
                mime_type=mime_type,
                file_size=file_size,
                **__pydantic_kwargs,
            )
