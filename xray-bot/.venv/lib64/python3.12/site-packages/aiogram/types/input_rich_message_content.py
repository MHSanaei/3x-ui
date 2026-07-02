from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .input_message_content import InputMessageContent

if TYPE_CHECKING:
    from .input_rich_message import InputRichMessage


class InputRichMessageContent(InputMessageContent):
    """
    Represents the `content <https://core.telegram.org/bots/api#inputmessagecontent>`_ of a rich message to be sent as the result of an inline query.

    Source: https://core.telegram.org/bots/api#inputrichmessagecontent
    """

    rich_message: InputRichMessage
    """The message to be sent"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, rich_message: InputRichMessage, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(rich_message=rich_message, **__pydantic_kwargs)
