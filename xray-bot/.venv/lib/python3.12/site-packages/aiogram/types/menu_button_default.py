from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import MenuButtonType
from .menu_button import MenuButton


class MenuButtonDefault(MenuButton):
    """
    Describes that no specific value for the menu button was set.

    Source: https://core.telegram.org/bots/api#menubuttondefault
    """

    type: Literal[MenuButtonType.DEFAULT] = MenuButtonType.DEFAULT
    """Type of the button, must be *default*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[MenuButtonType.DEFAULT] = MenuButtonType.DEFAULT,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
