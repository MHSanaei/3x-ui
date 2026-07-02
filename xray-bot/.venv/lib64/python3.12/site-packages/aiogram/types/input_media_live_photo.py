from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import InputMediaType
from .input_media import InputMedia
from .input_poll_media import InputPollMedia
from .input_poll_option_media import InputPollOptionMedia

if TYPE_CHECKING:
    from .message_entity import MessageEntity


class InputMediaLivePhoto(InputMedia, InputPollMedia, InputPollOptionMedia):
    """
    Represents a live photo to be sent.

    Source: https://core.telegram.org/bots/api#inputmedialivephoto
    """

    type: Literal[InputMediaType.LIVE_PHOTO] = InputMediaType.LIVE_PHOTO
    """Type of the media, must be *live_photo*"""
    media: str
    """Video of the live photo to send. Pass a file_id to send a file that exists on the Telegram servers (recommended) or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""
    photo: str
    """The static photo to send. Pass a file_id to send a file that exists on the Telegram servers (recommended) or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""
    caption: str | None = None
    """*Optional*. Caption of the live photo to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | None = None
    """*Optional*. Mode for parsing entities in the live photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | None = None
    """*Optional*. Pass :code:`True`, if the caption must be shown above the message media"""
    has_spoiler: bool | None = None
    """*Optional*. Pass :code:`True` if the live photo needs to be covered with a spoiler animation"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputMediaType.LIVE_PHOTO] = InputMediaType.LIVE_PHOTO,
            media: str,
            photo: str,
            caption: str | None = None,
            parse_mode: str | None = None,
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | None = None,
            has_spoiler: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                media=media,
                photo=photo,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                has_spoiler=has_spoiler,
                **__pydantic_kwargs,
            )
