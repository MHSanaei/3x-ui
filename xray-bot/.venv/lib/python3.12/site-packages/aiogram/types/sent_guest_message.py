from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class SentGuestMessage(TelegramObject):
    """
    Describes an inline message sent by a guest bot.

    Source: https://core.telegram.org/bots/api#sentguestmessage
    """

    inline_message_id: str
    """Identifier of the sent inline message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, inline_message_id: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(inline_message_id=inline_message_id, **__pydantic_kwargs)
