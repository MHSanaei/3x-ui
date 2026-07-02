from typing import TYPE_CHECKING, Any

from ..types import KeyboardButton, PreparedKeyboardButton
from .base import TelegramMethod


class SavePreparedKeyboardButton(TelegramMethod[PreparedKeyboardButton]):
    """
    Stores a keyboard button that can be used by a user within a Mini App. Returns a :class:`aiogram.types.prepared_keyboard_button.PreparedKeyboardButton` object.

    Source: https://core.telegram.org/bots/api#savepreparedkeyboardbutton
    """

    __returning__ = PreparedKeyboardButton
    __api_method__ = "savePreparedKeyboardButton"

    user_id: int
    """Unique identifier of the target user that can use the button"""
    button: KeyboardButton
    """A JSON-serialized object describing the button to be saved. The button must be of the type *request_users*, *request_chat*, or *request_managed_bot*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, user_id: int, button: KeyboardButton, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, button=button, **__pydantic_kwargs)
