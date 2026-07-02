from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class ChatOwnerLeft(TelegramObject):
    """
    Describes a service message about the chat owner leaving the chat.

    Source: https://core.telegram.org/bots/api#chatownerleft
    """

    new_owner: User | None = None
    """*Optional*. The user who will become the new owner of the chat if the previous owner does not return to the chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, new_owner: User | None = None, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(new_owner=new_owner, **__pydantic_kwargs)
