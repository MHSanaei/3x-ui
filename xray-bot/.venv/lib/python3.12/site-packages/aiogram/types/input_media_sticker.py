from typing import TYPE_CHECKING, Any, Literal

from ..enums import InputMediaType
from .base import TelegramObject
from .input_poll_option_media import InputPollOptionMedia


class InputMediaSticker(InputPollOptionMedia):
    """
    Represents a sticker file to be sent.

    Source: https://core.telegram.org/bots/api#inputmediasticker
    """

    type: Literal[InputMediaType.STICKER] = InputMediaType.STICKER
    """Type of the media, must be *sticker*"""
    media: str
    """File to send. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a .WEBP sticker from the Internet, or pass 'attach://<file_attach_name>' to upload a new .WEBP, .TGS, or .WEBM sticker using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`"""
    emoji: str | None = None
    """*Optional*. Emoji associated with the sticker; only for just uploaded stickers"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputMediaType.STICKER] = InputMediaType.STICKER,
            media: str,
            emoji: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, media=media, emoji=emoji, **__pydantic_kwargs)
