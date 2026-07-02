from .base import TelegramObject


class PaidMedia(TelegramObject):
    """
    This object describes paid media. Currently, it can be one of

     - :class:`aiogram.types.paid_media_live_photo.PaidMediaLivePhoto`
     - :class:`aiogram.types.paid_media_photo.PaidMediaPhoto`
     - :class:`aiogram.types.paid_media_preview.PaidMediaPreview`
     - :class:`aiogram.types.paid_media_video.PaidMediaVideo`

    Source: https://core.telegram.org/bots/api#paidmedia
    """
