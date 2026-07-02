from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .input_file_union import InputFileUnion
    from .mask_position import MaskPosition


class InputSticker(TelegramObject):
    """
    This object describes a sticker to be added to a sticker set.

    Source: https://core.telegram.org/bots/api#inputsticker
    """

    sticker: InputFileUnion
    """The added sticker. Pass a *file_id* as a String to send a file that already exists on the Telegram servers, pass an HTTP URL as a String for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new file using multipart/form-data under <file_attach_name> name. Animated and video stickers can't be uploaded via HTTP URL. :ref:`More information on Sending Files » <sending-files>`"""
    format: str
    """Format of the added sticker, must be one of 'static' for a **.WEBP** or **.PNG** image, 'animated' for a **.TGS** animation, 'video' for a **.WEBM** video"""
    emoji_list: list[str]
    """List of 1-20 emoji associated with the sticker"""
    mask_position: MaskPosition | None = None
    """*Optional*. Position where the mask should be placed on faces. For 'mask' stickers only"""
    keywords: list[str] | None = None
    """*Optional*. List of 0-20 search keywords for the sticker with total length of up to 64 characters. For 'regular' and 'custom_emoji' stickers only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            sticker: InputFileUnion,
            format: str,
            emoji_list: list[str],
            mask_position: MaskPosition | None = None,
            keywords: list[str] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                sticker=sticker,
                format=format,
                emoji_list=emoji_list,
                mask_position=mask_position,
                keywords=keywords,
                **__pydantic_kwargs,
            )
