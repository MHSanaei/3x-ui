from __future__ import annotations

from typing import TYPE_CHECKING, Any

from aiogram.types import BusinessConnection

from .base import TelegramMethod


class GetBusinessConnection(TelegramMethod[BusinessConnection]):
    """
    Use this method to get information about the connection of the bot with a business account. Returns a :class:`aiogram.types.business_connection.BusinessConnection` object on success.

    Source: https://core.telegram.org/bots/api#getbusinessconnection
    """

    __returning__ = BusinessConnection
    __api_method__ = "getBusinessConnection"

    business_connection_id: str
    """Unique identifier of the business connection"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, business_connection_id: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(business_connection_id=business_connection_id, **__pydantic_kwargs)
