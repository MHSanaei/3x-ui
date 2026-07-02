from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class ProximityAlertTriggered(TelegramObject):
    """
    This object represents the content of a service message, sent whenever a user in the chat triggers a proximity alert set by another user.

    Source: https://core.telegram.org/bots/api#proximityalerttriggered
    """

    traveler: User
    """User that triggered the alert"""
    watcher: User
    """User that set the alert"""
    distance: int
    """The distance between the users"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            traveler: User,
            watcher: User,
            distance: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                traveler=traveler, watcher=watcher, distance=distance, **__pydantic_kwargs
            )
