from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class BotName(TelegramObject):
    """
    This object represents the bot's name.

    Source: https://core.telegram.org/bots/api#botname
    """

    name: str
    """The bot's name"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, name: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, **__pydantic_kwargs)
