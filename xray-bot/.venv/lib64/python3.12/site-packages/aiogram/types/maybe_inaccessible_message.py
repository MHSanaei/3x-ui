from aiogram.types import TelegramObject


class MaybeInaccessibleMessage(TelegramObject):
    """
    This object describes a message that can be inaccessible to the bot. It can be one of

     - :class:`aiogram.types.message.Message`
     - :class:`aiogram.types.inaccessible_message.InaccessibleMessage`

    Source: https://core.telegram.org/bots/api#maybeinaccessiblemessage
    """
