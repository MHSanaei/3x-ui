from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from .base import MutableTelegramObject


class ReplyKeyboardRemove(MutableTelegramObject):
    """
    Upon receiving a message with this object, Telegram clients will remove the current custom keyboard and display the default letter-keyboard. By default, custom keyboards are displayed until a new keyboard is sent by a bot. An exception is made for one-time keyboards that are hidden immediately after the user presses a button (see :class:`aiogram.types.reply_keyboard_markup.ReplyKeyboardMarkup`). Not supported in channels and for messages sent on behalf of a business account.

    Source: https://core.telegram.org/bots/api#replykeyboardremove
    """

    remove_keyboard: Literal[True] = True
    """Requests clients to remove the custom keyboard (user will not be able to summon this keyboard; if you want to hide the keyboard from sight but keep it accessible, use *one_time_keyboard* in :class:`aiogram.types.reply_keyboard_markup.ReplyKeyboardMarkup`)"""
    selective: bool | None = None
    """*Optional*. Use this parameter if you want to remove the keyboard for specific users only. Targets: 1) users that are @mentioned in the *text* of the :class:`aiogram.types.message.Message` object; 2) if the bot's message is a reply to a message in the same chat and forum topic, sender of the original message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            remove_keyboard: Literal[True] = True,
            selective: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                remove_keyboard=remove_keyboard, selective=selective, **__pydantic_kwargs
            )
