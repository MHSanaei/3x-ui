from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import InputStoryContentType

from .input_story_content import InputStoryContent


class InputStoryContentVideo(InputStoryContent):
    """
    Describes a video to post as a story.

    Source: https://core.telegram.org/bots/api#inputstorycontentvideo
    """

    type: Literal[InputStoryContentType.VIDEO] = InputStoryContentType.VIDEO
    """Type of the content, must be *video*"""
    video: str
    """The video to post as a story. The video must be of the size 720x1280, streamable, encoded with H.265 codec, with key frames added each second in the MPEG4 format, and must not exceed 30 MB. The video can't be reused and can only be uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the video was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`"""
    duration: float | None = None
    """*Optional*. Precise duration of the video in seconds; 0-60"""
    cover_frame_timestamp: float | None = None
    """*Optional*. Timestamp in seconds of the frame that will be used as the static cover for the story. Defaults to 0.0"""
    is_animation: bool | None = None
    """*Optional*. Pass :code:`True` if the video has no sound"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputStoryContentType.VIDEO] = InputStoryContentType.VIDEO,
            video: str,
            duration: float | None = None,
            cover_frame_timestamp: float | None = None,
            is_animation: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                video=video,
                duration=duration,
                cover_frame_timestamp=cover_frame_timestamp,
                is_animation=is_animation,
                **__pydantic_kwargs,
            )
