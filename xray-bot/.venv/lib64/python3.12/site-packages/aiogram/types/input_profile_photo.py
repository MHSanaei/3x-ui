from __future__ import annotations

from .base import TelegramObject


class InputProfilePhoto(TelegramObject):
    """
    This object describes a profile photo to set. Currently, it can be one of

     - :class:`aiogram.types.input_profile_photo_static.InputProfilePhotoStatic`
     - :class:`aiogram.types.input_profile_photo_animated.InputProfilePhotoAnimated`

    Source: https://core.telegram.org/bots/api#inputprofilephoto
    """
