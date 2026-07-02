from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class File(TelegramObject):
    """
    This object represents a file ready to be downloaded. The file can be downloaded via the link :code:`https://api.telegram.org/file/bot<token>/<file_path>`. It is guaranteed that the link will be valid for at least 1 hour. When the link expires, a new one can be requested by calling :class:`aiogram.methods.get_file.GetFile`.

     The maximum file size to download is 20 MB

    Source: https://core.telegram.org/bots/api#file
    """

    file_id: str
    """Identifier for this file, which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for this file, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    file_size: int | None = None
    """*Optional*. File size in bytes. It can be bigger than 2^31 and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a signed 64-bit integer or double-precision float type are safe for storing this value"""
    file_path: str | None = None
    """*Optional*. File path. Use :code:`https://api.telegram.org/file/bot<token>/<file_path>` to get the file"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            file_id: str,
            file_unique_id: str,
            file_size: int | None = None,
            file_path: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                file_id=file_id,
                file_unique_id=file_unique_id,
                file_size=file_size,
                file_path=file_path,
                **__pydantic_kwargs,
            )
