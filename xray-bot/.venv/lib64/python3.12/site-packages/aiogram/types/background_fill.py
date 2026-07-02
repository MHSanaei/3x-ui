from .base import TelegramObject


class BackgroundFill(TelegramObject):
    """
    This object describes the way a background is filled based on the selected colors. Currently, it can be one of

     - :class:`aiogram.types.background_fill_solid.BackgroundFillSolid`
     - :class:`aiogram.types.background_fill_gradient.BackgroundFillGradient`
     - :class:`aiogram.types.background_fill_freeform_gradient.BackgroundFillFreeformGradient`

    Source: https://core.telegram.org/bots/api#backgroundfill
    """
