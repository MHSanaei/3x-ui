from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import MenuButtonUnion
from .base import TelegramMethod


class SetChatMenuButton(TelegramMethod[bool]):
    """
    Use this method to change the bot's menu button in a private chat, or the default menu button. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setchatmenubutton
    """

    __returning__ = bool
    __api_method__ = "setChatMenuButton"

    chat_id: int | None = None
    """Unique identifier for the target private chat. If not specified, the bot's default menu button will be changed"""
    menu_button: MenuButtonUnion | None = None
    """A JSON-serialized object for the bot's new menu button. Defaults to :class:`aiogram.types.menu_button_default.MenuButtonDefault`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: int | None = None,
            menu_button: MenuButtonUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, menu_button=menu_button, **__pydantic_kwargs)
