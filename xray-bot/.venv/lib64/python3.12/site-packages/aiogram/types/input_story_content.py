from __future__ import annotations

from .base import TelegramObject


class InputStoryContent(TelegramObject):
    """
    This object describes the content of a story to post. Currently, it can be one of

     - :class:`aiogram.types.input_story_content_photo.InputStoryContentPhoto`
     - :class:`aiogram.types.input_story_content_video.InputStoryContentVideo`

    Source: https://core.telegram.org/bots/api#inputstorycontent
    """
