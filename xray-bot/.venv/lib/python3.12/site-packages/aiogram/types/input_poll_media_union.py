from typing import TypeAlias

from .input_media_animation import InputMediaAnimation
from .input_media_audio import InputMediaAudio
from .input_media_document import InputMediaDocument
from .input_media_live_photo import InputMediaLivePhoto
from .input_media_location import InputMediaLocation
from .input_media_photo import InputMediaPhoto
from .input_media_venue import InputMediaVenue
from .input_media_video import InputMediaVideo

InputPollMediaUnion: TypeAlias = (
    InputMediaAnimation
    | InputMediaAudio
    | InputMediaDocument
    | InputMediaLivePhoto
    | InputMediaLocation
    | InputMediaPhoto
    | InputMediaVenue
    | InputMediaVideo
)
