from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class SentWebAppMessage(TelegramObject):
    """
    Describes an inline message sent by a `Web App <https://core.telegram.org/bots/webapps>`_ on behalf of a user.

    Source: https://core.telegram.org/bots/api#sentwebappmessage
    """

    inline_message_id: str | None = None
    """*Optional*. Identifier of the sent inline message. Available only if there is an `inline keyboard <https://core.telegram.org/bots/api#inlinekeyboardmarkup>`_ attached to the message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, inline_message_id: str | None = None, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(inline_message_id=inline_message_id, **__pydantic_kwargs)
