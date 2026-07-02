from aiogram.types import TelegramObject


class MessageOrigin(TelegramObject):
    """
    This object describes the origin of a message. It can be one of

     - :class:`aiogram.types.message_origin_user.MessageOriginUser`
     - :class:`aiogram.types.message_origin_hidden_user.MessageOriginHiddenUser`
     - :class:`aiogram.types.message_origin_chat.MessageOriginChat`
     - :class:`aiogram.types.message_origin_channel.MessageOriginChannel`

    Source: https://core.telegram.org/bots/api#messageorigin
    """
