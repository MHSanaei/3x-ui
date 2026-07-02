from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class CopyTextButton(TelegramObject):
    """
    This object represents an inline keyboard button that copies specified text to the clipboard.

    Source: https://core.telegram.org/bots/api#copytextbutton
    """

    text: str
    """The text to be copied to the clipboard; 1-256 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, text: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(text=text, **__pydantic_kwargs)
