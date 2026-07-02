from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject

if TYPE_CHECKING:
    from .inline_keyboard_button import InlineKeyboardButton


class InlineKeyboardMarkup(MutableTelegramObject):
    """
    This object represents an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ that appears right next to the message it belongs to.

    Source: https://core.telegram.org/bots/api#inlinekeyboardmarkup
    """

    inline_keyboard: list[list[InlineKeyboardButton]]
    """Array of button rows, each represented by an Array of :class:`aiogram.types.inline_keyboard_button.InlineKeyboardButton` objects"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            inline_keyboard: list[list[InlineKeyboardButton]],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(inline_keyboard=inline_keyboard, **__pydantic_kwargs)
