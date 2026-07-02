from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InputMediaType
from .input_media import InputMedia
from .input_poll_media import InputPollMedia

if TYPE_CHECKING:
    from .input_file import InputFile
    from .input_file_union import InputFileUnion
    from .message_entity import MessageEntity


class InputMediaDocument(InputMedia, InputPollMedia):
    """
    Represents a general file to be sent.

    Source: https://core.telegram.org/bots/api#inputmediadocument
    """

    type: Literal[InputMediaType.DOCUMENT] = InputMediaType.DOCUMENT
    """Type of the media, must be *document*"""
    media: InputFileUnion
    """File to send. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`"""
    thumbnail: InputFile | None = None
    """*Optional*. Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`"""
    caption: str | None = None
    """*Optional*. Caption of the document to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    disable_content_type_detection: bool | None = None
    """*Optional*. Disables automatic server-side content type detection for files uploaded using multipart/form-data. Always :code:`True`, if the document is sent as part of an album"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputMediaType.DOCUMENT] = InputMediaType.DOCUMENT,
            media: InputFileUnion,
            thumbnail: InputFile | None = None,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            disable_content_type_detection: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                media=media,
                thumbnail=thumbnail,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                disable_content_type_detection=disable_content_type_detection,
                **__pydantic_kwargs,
            )
