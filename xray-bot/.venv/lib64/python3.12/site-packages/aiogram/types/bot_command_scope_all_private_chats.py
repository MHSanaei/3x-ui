from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import BotCommandScopeType
from .bot_command_scope import BotCommandScope


class BotCommandScopeAllPrivateChats(BotCommandScope):
    """
    Represents the `scope <https://core.telegram.org/bots/api#botcommandscope>`_ of bot commands, covering all private chats.

    Source: https://core.telegram.org/bots/api#botcommandscopeallprivatechats
    """

    type: Literal[BotCommandScopeType.ALL_PRIVATE_CHATS] = BotCommandScopeType.ALL_PRIVATE_CHATS
    """Scope type, must be *all_private_chats*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                BotCommandScopeType.ALL_PRIVATE_CHATS
            ] = BotCommandScopeType.ALL_PRIVATE_CHATS,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
