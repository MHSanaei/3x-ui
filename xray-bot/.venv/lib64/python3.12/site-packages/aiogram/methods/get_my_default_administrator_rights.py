from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatAdministratorRights
from .base import TelegramMethod


class GetMyDefaultAdministratorRights(TelegramMethod[ChatAdministratorRights]):
    """
    Use this method to get the current default administrator rights of the bot. Returns :class:`aiogram.types.chat_administrator_rights.ChatAdministratorRights` on success.

    Source: https://core.telegram.org/bots/api#getmydefaultadministratorrights
    """

    __returning__ = ChatAdministratorRights
    __api_method__ = "getMyDefaultAdministratorRights"

    for_channels: bool | None = None
    """Pass :code:`True` to get default administrator rights of the bot in channels. Otherwise, default administrator rights of the bot for groups and supergroups will be returned"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, for_channels: bool | None = None, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(for_channels=for_channels, **__pydantic_kwargs)
