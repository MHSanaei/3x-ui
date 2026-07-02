from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import BotCommandScopeType
from .bot_command_scope import BotCommandScope

if TYPE_CHECKING:
    from .chat_id_union import ChatIdUnion


class BotCommandScopeChatAdministrators(BotCommandScope):
    """
    Represents the `scope <https://core.telegram.org/bots/api#botcommandscope>`_ of bot commands, covering all administrators of a specific group or supergroup chat.

    Source: https://core.telegram.org/bots/api#botcommandscopechatadministrators
    """

    type: Literal[BotCommandScopeType.CHAT_ADMINISTRATORS] = (
        BotCommandScopeType.CHAT_ADMINISTRATORS
    )
    """Scope type, must be *chat_administrators*"""
    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`. Channel direct messages chats and channel chats aren't supported"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                BotCommandScopeType.CHAT_ADMINISTRATORS
            ] = BotCommandScopeType.CHAT_ADMINISTRATORS,
            chat_id: ChatIdUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, chat_id=chat_id, **__pydantic_kwargs)
