from __future__ import annotations

from .base import MutableTelegramObject


class InputMedia(MutableTelegramObject):
    """
    This object represents the content of a media message to be sent. It should be one of

     - :class:`aiogram.types.input_media_animation.InputMediaAnimation`
     - :class:`aiogram.types.input_media_audio.InputMediaAudio`
     - :class:`aiogram.types.input_media_document.InputMediaDocument`
     - :class:`aiogram.types.input_media_live_photo.InputMediaLivePhoto`
     - :class:`aiogram.types.input_media_photo.InputMediaPhoto`
     - :class:`aiogram.types.input_media_video.InputMediaVideo`

    Source: https://core.telegram.org/bots/api#inputmedia
    """
