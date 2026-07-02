from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import InputProfilePhotoType

from .input_file_union import InputFileUnion
from .input_profile_photo import InputProfilePhoto


class InputProfilePhotoAnimated(InputProfilePhoto):
    """
    An animated profile photo in the MPEG4 format.

    Source: https://core.telegram.org/bots/api#inputprofilephotoanimated
    """

    type: Literal[InputProfilePhotoType.ANIMATED] = InputProfilePhotoType.ANIMATED
    """Type of the profile photo, must be *animated*"""
    animation: InputFileUnion
    """The animated profile photo. Profile photos can't be reused and can only be uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the photo was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`"""
    main_frame_timestamp: float | None = None
    """*Optional*. Timestamp in seconds of the frame that will be used as the static profile photo. Defaults to 0.0"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputProfilePhotoType.ANIMATED] = InputProfilePhotoType.ANIMATED,
            animation: InputFileUnion,
            main_frame_timestamp: float | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                animation=animation,
                main_frame_timestamp=main_frame_timestamp,
                **__pydantic_kwargs,
            )
