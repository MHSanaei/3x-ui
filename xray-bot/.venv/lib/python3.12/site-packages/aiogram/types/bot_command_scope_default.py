from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import BotCommandScopeType
from .bot_command_scope import BotCommandScope


class BotCommandScopeDefault(BotCommandScope):
    """
    Represents the default `scope <https://core.telegram.org/bots/api#botcommandscope>`_ of bot commands. Default commands are used if no commands with a `narrower scope <https://core.telegram.org/bots/api#determining-list-of-commands>`_ are specified for the user.

    Source: https://core.telegram.org/bots/api#botcommandscopedefault
    """

    type: Literal[BotCommandScopeType.DEFAULT] = BotCommandScopeType.DEFAULT
    """Scope type, must be *default*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[BotCommandScopeType.DEFAULT] = BotCommandScopeType.DEFAULT,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
