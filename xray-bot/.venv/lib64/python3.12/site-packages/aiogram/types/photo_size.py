from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class PhotoSize(TelegramObject):
    """
    This object represents one size of a photo or a `file <https://core.telegram.org/bots/api#document>`_ / :class:`aiogram.methods.sticker.Sticker` thumbnail.

    Source: https://core.telegram.org/bots/api#photosize
    """

    file_id: str
    """Identifier for this file, which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for this file, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    width: int
    """Photo width"""
    height: int
    """Photo height"""
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
            width: int,
            height: int,
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
                file_size=file_size,
                **__pydantic_kwargs,
            )
