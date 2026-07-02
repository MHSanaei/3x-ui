from .base import TelegramObject


class InputPollMedia(TelegramObject):
    """
    This object represents the content of a poll description or a quiz explanation to be sent. It should be one of

     - :class:`aiogram.types.input_media_animation.InputMediaAnimation`
     - :class:`aiogram.types.input_media_audio.InputMediaAudio`
     - :class:`aiogram.types.input_media_document.InputMediaDocument`
     - :class:`aiogram.types.input_media_live_photo.InputMediaLivePhoto`
     - :class:`aiogram.types.input_media_location.InputMediaLocation`
     - :class:`aiogram.types.input_media_photo.InputMediaPhoto`
     - :class:`aiogram.types.input_media_venue.InputMediaVenue`
     - :class:`aiogram.types.input_media_video.InputMediaVideo`

    Source: https://core.telegram.org/bots/api#inputpollmedia
    """
