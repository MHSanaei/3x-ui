from aiogram.types import TelegramObject


class ChatBoostSource(TelegramObject):
    """
    This object describes the source of a chat boost. It can be one of

     - :class:`aiogram.types.chat_boost_source_premium.ChatBoostSourcePremium`
     - :class:`aiogram.types.chat_boost_source_gift_code.ChatBoostSourceGiftCode`
     - :class:`aiogram.types.chat_boost_source_giveaway.ChatBoostSourceGiveaway`

    Source: https://core.telegram.org/bots/api#chatboostsource
    """
