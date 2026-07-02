from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class DirectMessagesTopic(TelegramObject):
    """
    Describes a topic of a direct messages chat.

    Source: https://core.telegram.org/bots/api#directmessagestopic
    """

    topic_id: int
    """Unique identifier of the topic. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a 64-bit integer or double-precision float type are safe for storing this identifier"""
    user: User | None = None
    """*Optional*. Information about the user that created the topic. Currently, it is always present"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            topic_id: int,
            user: User | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(topic_id=topic_id, user=user, **__pydantic_kwargs)
