from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .location import Location


class ChatLocation(TelegramObject):
    """
    Represents a location to which a chat is connected.

    Source: https://core.telegram.org/bots/api#chatlocation
    """

    location: Location
    """The location to which the supergroup is connected. Can't be a live location"""
    address: str
    """Location address; 1-64 characters, as defined by the chat owner"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, location: Location, address: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(location=location, address=address, **__pydantic_kwargs)
