from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import BotCommandScopeType
from .bot_command_scope import BotCommandScope


class BotCommandScopeAllChatAdministrators(BotCommandScope):
    """
    Represents the `scope <https://core.telegram.org/bots/api#botcommandscope>`_ of bot commands, covering all group and supergroup chat administrators.

    Source: https://core.telegram.org/bots/api#botcommandscopeallchatadministrators
    """

    type: Literal[BotCommandScopeType.ALL_CHAT_ADMINISTRATORS] = (
        BotCommandScopeType.ALL_CHAT_ADMINISTRATORS
    )
    """Scope type, must be *all_chat_administrators*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                BotCommandScopeType.ALL_CHAT_ADMINISTRATORS
            ] = BotCommandScopeType.ALL_CHAT_ADMINISTRATORS,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
