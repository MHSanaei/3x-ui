from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from .shared_user import SharedUser


class UsersShared(TelegramObject):
    """
    This object contains information about the users whose identifiers were shared with the bot using a :class:`aiogram.types.keyboard_button_request_users.KeyboardButtonRequestUsers` button.

    Source: https://core.telegram.org/bots/api#usersshared
    """

    request_id: int
    """Identifier of the request"""
    users: list[SharedUser]
    """Information about users shared with the bot"""
    user_ids: list[int] | None = Field(None, json_schema_extra={"deprecated": True})
    """Identifiers of the shared users. These numbers may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting them. But they have at most 52 significant bits, so 64-bit integers or double-precision float types are safe for storing these identifiers. The bot may not have access to the users and could be unable to use these identifiers, unless the users are already known to the bot by some other means

.. deprecated:: API:7.2
   https://core.telegram.org/bots/api-changelog#march-31-2024"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            request_id: int,
            users: list[SharedUser],
            user_ids: list[int] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                request_id=request_id, users=users, user_ids=user_ids, **__pydantic_kwargs
            )
