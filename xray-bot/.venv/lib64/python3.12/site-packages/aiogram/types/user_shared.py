from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject


class UserShared(TelegramObject):
    """
    This object contains information about the user whose identifier was shared with the bot using a :class:`aiogram.types.keyboard_button_request_user.KeyboardButtonRequestUser` button.

    .. deprecated:: API:7.0
       https://core.telegram.org/bots/api-changelog#december-29-2023

    Source: https://core.telegram.org/bots/api#usershared
    """

    request_id: int
    """Identifier of the request"""
    user_id: int
    """Identifier of the shared user. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a 64-bit integer or double-precision float type are safe for storing this identifier. The bot may not have access to the user and could be unable to use this identifier, unless the user is already known to the bot by some other means"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, request_id: int, user_id: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(request_id=request_id, user_id=user_id, **__pydantic_kwargs)
