from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class SharedUser(TelegramObject):
    """
    This object contains information about a user that was shared with the bot using a :class:`aiogram.types.keyboard_button_request_users.KeyboardButtonRequestUsers` button.

    Source: https://core.telegram.org/bots/api#shareduser
    """

    user_id: int
    """Identifier of the shared user. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so 64-bit integers or double-precision float types are safe for storing these identifiers. The bot may not have access to the user and could be unable to use this identifier, unless the user is already known to the bot by some other means"""
    first_name: str | None = None
    """*Optional*. First name of the user, if the name was requested by the bot"""
    last_name: str | None = None
    """*Optional*. Last name of the user, if the name was requested by the bot"""
    username: str | None = None
    """*Optional*. Username of the user, if the username was requested by the bot"""
    photo: list[PhotoSize] | None = None
    """*Optional*. Available sizes of the chat photo, if the photo was requested by the bot"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            first_name: str | None = None,
            last_name: str | None = None,
            username: str | None = None,
            photo: list[PhotoSize] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                first_name=first_name,
                last_name=last_name,
                username=username,
                photo=photo,
                **__pydantic_kwargs,
            )
