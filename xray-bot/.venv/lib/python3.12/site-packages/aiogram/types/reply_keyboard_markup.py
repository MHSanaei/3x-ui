from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject

if TYPE_CHECKING:
    from .keyboard_button import KeyboardButton


class ReplyKeyboardMarkup(MutableTelegramObject):
    """
    This object represents a `custom keyboard <https://core.telegram.org/bots/features#keyboards>`_ with reply options (see `Introduction to bots <https://core.telegram.org/bots/features#keyboards>`_ for details and examples). Not supported in channels and for messages sent on behalf of a business account.

    Source: https://core.telegram.org/bots/api#replykeyboardmarkup
    """

    keyboard: list[list[KeyboardButton]]
    """Array of button rows, each represented by an Array of :class:`aiogram.types.keyboard_button.KeyboardButton` objects"""
    is_persistent: bool | None = None
    """*Optional*. Requests clients to always show the keyboard when the regular keyboard is hidden. Defaults to *false*, in which case the custom keyboard can be hidden and opened with a keyboard icon"""
    resize_keyboard: bool | None = None
    """*Optional*. Requests clients to resize the keyboard vertically for optimal fit (e.g., make the keyboard smaller if there are just two rows of buttons). Defaults to *false*, in which case the custom keyboard is always of the same height as the app's standard keyboard"""
    one_time_keyboard: bool | None = None
    """*Optional*. Requests clients to hide the keyboard as soon as it's been used. The keyboard will still be available, but clients will automatically display the usual letter-keyboard in the chat - the user can press a special button in the input field to see the custom keyboard again. Defaults to *false*"""
    input_field_placeholder: str | None = None
    """*Optional*. The placeholder to be shown in the input field when the keyboard is active; 1-64 characters"""
    selective: bool | None = None
    """*Optional*. Use this parameter if you want to show the keyboard to specific users only. Targets: 1) users that are @mentioned in the *text* of the :class:`aiogram.types.message.Message` object; 2) if the bot's message is a reply to a message in the same chat and forum topic, sender of the original message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            keyboard: list[list[KeyboardButton]],
            is_persistent: bool | None = None,
            resize_keyboard: bool | None = None,
            one_time_keyboard: bool | None = None,
            input_field_placeholder: str | None = None,
            selective: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                keyboard=keyboard,
                is_persistent=is_persistent,
                resize_keyboard=resize_keyboard,
                one_time_keyboard=one_time_keyboard,
                input_field_placeholder=input_field_placeholder,
                selective=selective,
                **__pydantic_kwargs,
            )
