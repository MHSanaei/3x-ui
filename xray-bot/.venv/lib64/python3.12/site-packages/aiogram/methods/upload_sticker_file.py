from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import File, InputFile
from .base import TelegramMethod


class UploadStickerFile(TelegramMethod[File]):
    """
    Use this method to upload a file with a sticker for later use in the :class:`aiogram.methods.create_new_sticker_set.CreateNewStickerSet`, :class:`aiogram.methods.add_sticker_to_set.AddStickerToSet`, or :class:`aiogram.methods.replace_sticker_in_set.ReplaceStickerInSet` methods (the file can be used multiple times). Returns the uploaded :class:`aiogram.types.file.File` on success.

    Source: https://core.telegram.org/bots/api#uploadstickerfile
    """

    __returning__ = File
    __api_method__ = "uploadStickerFile"

    user_id: int
    """User identifier of sticker file owner"""
    sticker: InputFile
    """A file with the sticker in .WEBP, .PNG, .TGS, or .WEBM format. See `https://core.telegram.org/stickers <https://core.telegram.org/stickers>`_`https://core.telegram.org/stickers <https://core.telegram.org/stickers>`_ for technical requirements. :ref:`More information on Sending Files » <sending-files>`"""
    sticker_format: str
    """Format of the sticker, must be one of 'static', 'animated', 'video'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            sticker: InputFile,
            sticker_format: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                sticker=sticker,
                sticker_format=sticker_format,
                **__pydantic_kwargs,
            )
