from typing import TYPE_CHECKING, Any, Literal

from ..enums import MessageOriginType
from .custom import DateTime
from .message_origin import MessageOrigin


class MessageOriginHiddenUser(MessageOrigin):
    """
    The message was originally sent by an unknown user.

    Source: https://core.telegram.org/bots/api#messageoriginhiddenuser
    """

    type: Literal[MessageOriginType.HIDDEN_USER] = MessageOriginType.HIDDEN_USER
    """Type of the message origin, always 'hidden_user'"""
    date: DateTime
    """Date the message was sent originally in Unix time"""
    sender_user_name: str
    """Name of the user that sent the message originally"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MessageOriginType.HIDDEN_USER] = MessageOriginType.HIDDEN_USER,
            date: DateTime,
            sender_user_name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, date=date, sender_user_name=sender_user_name, **__pydantic_kwargs
            )
