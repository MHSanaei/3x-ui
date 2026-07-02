from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject


class BotDescription(TelegramObject):
    """
    This object represents the bot's description.

    Source: https://core.telegram.org/bots/api#botdescription
    """

    description: str
    """The bot's description"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, description: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(description=description, **__pydantic_kwargs)
