from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class ChatPhoto(TelegramObject):
    """
    This object represents a chat photo.

    Source: https://core.telegram.org/bots/api#chatphoto
    """

    small_file_id: str
    """File identifier of small (160x160) chat photo. This file_id can be used only for photo download and only for as long as the photo is not changed"""
    small_file_unique_id: str
    """Unique file identifier of small (160x160) chat photo, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    big_file_id: str
    """File identifier of big (640x640) chat photo. This file_id can be used only for photo download and only for as long as the photo is not changed"""
    big_file_unique_id: str
    """Unique file identifier of big (640x640) chat photo, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            small_file_id: str,
            small_file_unique_id: str,
            big_file_id: str,
            big_file_unique_id: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                small_file_id=small_file_id,
                small_file_unique_id=small_file_unique_id,
                big_file_id=big_file_id,
                big_file_unique_id=big_file_unique_id,
                **__pydantic_kwargs,
            )
