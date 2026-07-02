from .base import TelegramObject


class InputPaidMedia(TelegramObject):
    """
    This object describes the paid media to be sent. Currently, it can be one of

     - :class:`aiogram.types.input_paid_media_live_photo.InputPaidMediaLivePhoto`
     - :class:`aiogram.types.input_paid_media_photo.InputPaidMediaPhoto`
     - :class:`aiogram.types.input_paid_media_video.InputPaidMediaVideo`

    Source: https://core.telegram.org/bots/api#inputpaidmedia
    """
