from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import InputPaidMediaType
from .input_file_union import InputFileUnion
from .input_paid_media import InputPaidMedia


class InputPaidMediaPhoto(InputPaidMedia):
    """
    The paid media to send is a photo.

    Source: https://core.telegram.org/bots/api#inputpaidmediaphoto
    """

    type: Literal[InputPaidMediaType.PHOTO] = InputPaidMediaType.PHOTO
    """Type of the media, must be *photo*"""
    media: InputFileUnion
    """File to send. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputPaidMediaType.PHOTO] = InputPaidMediaType.PHOTO,
            media: InputFileUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, media=media, **__pydantic_kwargs)
