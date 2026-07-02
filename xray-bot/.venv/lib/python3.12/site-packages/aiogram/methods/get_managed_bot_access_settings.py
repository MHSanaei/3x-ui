from typing import TYPE_CHECKING, Any

from ..types.bot_access_settings import BotAccessSettings
from .base import TelegramMethod


class GetManagedBotAccessSettings(TelegramMethod[BotAccessSettings]):
    """
    Use this method to get the access settings of a managed bot. Returns a :class:`aiogram.types.bot_access_settings.BotAccessSettings` object on success.

    Source: https://core.telegram.org/bots/api#getmanagedbotaccesssettings
    """

    __returning__ = BotAccessSettings
    __api_method__ = "getManagedBotAccessSettings"

    user_id: int
    """User identifier of the managed bot whose access settings will be returned"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, user_id: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, **__pydantic_kwargs)
