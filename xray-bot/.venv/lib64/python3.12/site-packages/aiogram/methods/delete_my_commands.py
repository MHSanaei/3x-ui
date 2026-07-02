from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import BotCommandScopeUnion
from .base import TelegramMethod


class DeleteMyCommands(TelegramMethod[bool]):
    """
    Use this method to delete the list of the bot's commands for the given scope and user language. After deletion, `higher level commands <https://core.telegram.org/bots/api#determining-list-of-commands>`_ will be shown to affected users. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletemycommands
    """

    __returning__ = bool
    __api_method__ = "deleteMyCommands"

    scope: BotCommandScopeUnion | None = None
    """A JSON-serialized object, describing scope of users for which the commands are relevant. Defaults to :class:`aiogram.types.bot_command_scope_default.BotCommandScopeDefault`"""
    language_code: str | None = None
    """A two-letter ISO 639-1 language code. If empty, commands will be applied to all users from the given scope, for whose language there are no dedicated commands"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            scope: BotCommandScopeUnion | None = None,
            language_code: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(scope=scope, language_code=language_code, **__pydantic_kwargs)
