from typing import TypeAlias

from .input_media_audio import InputMediaAudio
from .input_media_document import InputMediaDocument
from .input_media_live_photo import InputMediaLivePhoto
from .input_media_photo import InputMediaPhoto
from .input_media_video import InputMediaVideo

MediaUnion: TypeAlias = (
    InputMediaAudio | InputMediaDocument | InputMediaLivePhoto | InputMediaPhoto | InputMediaVideo
)
