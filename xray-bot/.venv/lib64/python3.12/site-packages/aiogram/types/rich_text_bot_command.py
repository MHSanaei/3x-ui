from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextBotCommand(RichText):
    """
    A bot command.

    Source: https://core.telegram.org/bots/api#richtextbotcommand
    """

    type: Literal[RichTextType.BOT_COMMAND] = RichTextType.BOT_COMMAND
    """Type of the rich text, always 'bot_command'"""
    text: RichTextUnion
    """The text"""
    bot_command: str
    """The bot command"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.BOT_COMMAND] = RichTextType.BOT_COMMAND,
            text: RichTextUnion,
            bot_command: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, bot_command=bot_command, **__pydantic_kwargs)
