from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import MenuButtonType
from .menu_button import MenuButton

if TYPE_CHECKING:
    from .web_app_info import WebAppInfo


class MenuButtonWebApp(MenuButton):
    """
    Represents a menu button, which launches a `Web App <https://core.telegram.org/bots/webapps>`_.

    Source: https://core.telegram.org/bots/api#menubuttonwebapp
    """

    type: Literal[MenuButtonType.WEB_APP] = MenuButtonType.WEB_APP
    """Type of the button, must be *web_app*"""
    text: str
    """Text on the button"""
    web_app: WebAppInfo
    """Description of the Web App that will be launched when the user presses the button. The Web App will be able to send an arbitrary message on behalf of the user using the method :class:`aiogram.methods.answer_web_app_query.AnswerWebAppQuery`. Alternatively, a :code:`t.me` link to a Web App of the bot can be specified in the object instead of the Web App's URL, in which case the Web App will be opened as if the user pressed the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MenuButtonType.WEB_APP] = MenuButtonType.WEB_APP,
            text: str,
            web_app: WebAppInfo,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, web_app=web_app, **__pydantic_kwargs)
