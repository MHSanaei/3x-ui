from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class ChatOwnerChanged(TelegramObject):
    """
    Describes a service message about an ownership change in the chat.

    Source: https://core.telegram.org/bots/api#chatownerchanged
    """

    new_owner: User
    """The new owner of the chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, new_owner: User, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(new_owner=new_owner, **__pydantic_kwargs)
