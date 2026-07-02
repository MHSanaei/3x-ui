from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class WebAppData(TelegramObject):
    """
    Describes data sent from a `Web App <https://core.telegram.org/bots/webapps>`_ to the bot.

    Source: https://core.telegram.org/bots/api#webappdata
    """

    data: str
    """The data. Be aware that a bad client can send arbitrary data in this field"""
    button_text: str
    """Text of the *web_app* keyboard button from which the Web App was opened. Be aware that a bad client can send arbitrary data in this field"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, data: str, button_text: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(data=data, button_text=button_text, **__pydantic_kwargs)
