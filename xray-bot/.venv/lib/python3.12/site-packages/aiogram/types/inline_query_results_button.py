from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .web_app_info import WebAppInfo


class InlineQueryResultsButton(TelegramObject):
    """
    This object represents a button to be shown above inline query results. You **must** use exactly one of the optional fields.

    Source: https://core.telegram.org/bots/api#inlinequeryresultsbutton
    """

    text: str
    """Label text on the button"""
    web_app: WebAppInfo | None = None
    """*Optional*. Description of the `Web App <https://core.telegram.org/bots/webapps>`_ that will be launched when the user presses the button. The Web App will be able to switch back to the inline mode using the method `switchInlineQuery <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_ inside the Web App"""
    start_parameter: str | None = None
    """*Optional*. `Deep-linking <https://core.telegram.org/bots/features#deep-linking>`_ parameter for the /start message sent to the bot when a user presses the button. 1-64 characters, only :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: str,
            web_app: WebAppInfo | None = None,
            start_parameter: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                text=text, web_app=web_app, start_parameter=start_parameter, **__pydantic_kwargs
            )
