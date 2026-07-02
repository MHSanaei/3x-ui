from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputFileUnion
from .base import TelegramMethod


class SetStickerSetThumbnail(TelegramMethod[bool]):
    """
    Use this method to set the thumbnail of a regular or mask sticker set. The format of the thumbnail file must match the format of the stickers in the set. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickersetthumbnail
    """

    __returning__ = bool
    __api_method__ = "setStickerSetThumbnail"

    name: str
    """Sticker set name"""
    user_id: int
    """User identifier of the sticker set owner"""
    format: str
    """Format of the thumbnail, must be one of 'static' for a **.WEBP** or **.PNG** image, 'animated' for a **.TGS** animation, or 'video' for a **.WEBM** video"""
    thumbnail: InputFileUnion | None = None
    """A **.WEBP** or **.PNG** image with the thumbnail, must be up to 128 kilobytes in size and have a width and height of exactly 100px, or a **.TGS** animation with a thumbnail up to 32 kilobytes in size (see `https://core.telegram.org/stickers#animation-requirements <https://core.telegram.org/stickers#animation-requirements>`_`https://core.telegram.org/stickers#animation-requirements <https://core.telegram.org/stickers#animation-requirements>`_ for animated sticker technical requirements), or a **.WEBM** video with the thumbnail up to 32 kilobytes in size; see `https://core.telegram.org/stickers#video-requirements <https://core.telegram.org/stickers#video-requirements>`_`https://core.telegram.org/stickers#video-requirements <https://core.telegram.org/stickers#video-requirements>`_ for video sticker technical requirements. Pass a *file_id* as a String to send a file that already exists on the Telegram servers, pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Animated and video sticker set thumbnails can't be uploaded via HTTP URL. If omitted, then the thumbnail is dropped and the first sticker is used as the thumbnail"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str,
            user_id: int,
            format: str,
            thumbnail: InputFileUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                name=name, user_id=user_id, format=format, thumbnail=thumbnail, **__pydantic_kwargs
            )
