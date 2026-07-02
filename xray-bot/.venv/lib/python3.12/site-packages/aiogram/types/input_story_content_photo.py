from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import InputStoryContentType

from .input_story_content import InputStoryContent


class InputStoryContentPhoto(InputStoryContent):
    """
    Describes a photo to post as a story.

    Source: https://core.telegram.org/bots/api#inputstorycontentphoto
    """

    type: Literal[InputStoryContentType.PHOTO] = InputStoryContentType.PHOTO
    """Type of the content, must be *photo*"""
    photo: str
    """The photo to post as a story. The photo must be of the size 1080x1920 and must not exceed 10 MB. The photo can't be reused and can only be uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the photo was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputStoryContentType.PHOTO] = InputStoryContentType.PHOTO,
            photo: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, photo=photo, **__pydantic_kwargs)
