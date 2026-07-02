from .base import TelegramObject


class InputPollOptionMedia(TelegramObject):
    """
    This object represents the content of a poll option to be sent. It should be one of

     - :class:`aiogram.types.input_media_animation.InputMediaAnimation`
     - :class:`aiogram.types.input_media_link.InputMediaLink`
     - :class:`aiogram.types.input_media_live_photo.InputMediaLivePhoto`
     - :class:`aiogram.types.input_media_location.InputMediaLocation`
     - :class:`aiogram.types.input_media_photo.InputMediaPhoto`
     - :class:`aiogram.types.input_media_sticker.InputMediaSticker`
     - :class:`aiogram.types.input_media_venue.InputMediaVenue`
     - :class:`aiogram.types.input_media_video.InputMediaVideo`

    Source: https://core.telegram.org/bots/api#inputpolloptionmedia
    """
