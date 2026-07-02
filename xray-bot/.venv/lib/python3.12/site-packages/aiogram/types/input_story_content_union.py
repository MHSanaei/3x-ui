from typing import TypeAlias

from .input_story_content_photo import InputStoryContentPhoto
from .input_story_content_video import InputStoryContentVideo

InputStoryContentUnion: TypeAlias = InputStoryContentPhoto | InputStoryContentVideo
