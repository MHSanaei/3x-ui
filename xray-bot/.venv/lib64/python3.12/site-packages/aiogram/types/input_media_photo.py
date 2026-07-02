from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InputMediaType
from .input_media import InputMedia
from .input_poll_media import InputPollMedia
from .input_poll_option_media import InputPollOptionMedia

if TYPE_CHECKING:
    from .input_file_union import InputFileUnion
    from .message_entity import MessageEntity


class InputMediaPhoto(InputMedia, InputPollMedia, InputPollOptionMedia):
    """
    Represents a photo to be sent.

    Source: https://core.telegram.org/bots/api#inputmediaphoto
    """

    type: Literal[InputMediaType.PHOTO] = InputMediaType.PHOTO
    """Type of the media, must be *photo*"""
    media: InputFileUnion
    """File to send. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`"""
    caption: str | None = None
    """*Optional*. Caption of the photo to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | Default | None = Default("show_caption_above_media")
    """*Optional*. Pass :code:`True`, if the caption must be shown above the message media"""
    has_spoiler: bool | None = None
    """*Optional*. Pass :code:`True` if the photo needs to be covered with a spoiler animation"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputMediaType.PHOTO] = InputMediaType.PHOTO,
            media: InputFileUnion,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
            has_spoiler: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                media=media,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                has_spoiler=has_spoiler,
                **__pydantic_kwargs,
            )
