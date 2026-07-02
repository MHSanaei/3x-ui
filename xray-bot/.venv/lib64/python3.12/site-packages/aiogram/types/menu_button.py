from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject

if TYPE_CHECKING:
    from .web_app_info import WebAppInfo


class MenuButton(MutableTelegramObject):
    """
    This object describes the bot's menu button in a private chat. It should be one of

     - :class:`aiogram.types.menu_button_commands.MenuButtonCommands`
     - :class:`aiogram.types.menu_button_web_app.MenuButtonWebApp`
     - :class:`aiogram.types.menu_button_default.MenuButtonDefault`

    If a menu button other than :class:`aiogram.types.menu_button_default.MenuButtonDefault` is set for a private chat, then it is applied in the chat. Otherwise the default menu button is applied. By default, the menu button opens the list of bot commands.

    Source: https://core.telegram.org/bots/api#menubutton
    """

    type: str
    """Type of the button"""
    text: str | None = None
    """*Optional*. Text on the button"""
    web_app: WebAppInfo | None = None
    """*Optional*. Description of the Web App that will be launched when the user presses the button. The Web App will be able to send an arbitrary message on behalf of the user using the method :class:`aiogram.methods.answer_web_app_query.AnswerWebAppQuery`. Alternatively, a :code:`t.me` link to a Web App of the bot can be specified in the object instead of the Web App's URL, in which case the Web App will be opened as if the user pressed the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: str,
            text: str | None = None,
            web_app: WebAppInfo | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, web_app=web_app, **__pydantic_kwargs)
