from typing import TYPE_CHECKING, Any, Literal

from ..enums import InputPaidMediaType
from .input_paid_media import InputPaidMedia


class InputPaidMediaLivePhoto(InputPaidMedia):
    """
    The paid media to send is a live photo.

    Source: https://core.telegram.org/bots/api#inputpaidmedialivephoto
    """

    type: Literal[InputPaidMediaType.LIVE_PHOTO] = InputPaidMediaType.LIVE_PHOTO
    """Type of the media, must be *live_photo*"""
    media: str
    """Video of the live photo to send. Pass a file_id to send a file that exists on the Telegram servers (recommended) or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""
    photo: str
    """The static photo to send. Pass a file_id to send a file that exists on the Telegram servers (recommended) or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputPaidMediaType.LIVE_PHOTO] = InputPaidMediaType.LIVE_PHOTO,
            media: str,
            photo: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, media=media, photo=photo, **__pydantic_kwargs)
