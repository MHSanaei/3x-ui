from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class VideoChatEnded(TelegramObject):
    """
    This object represents a service message about a video chat ended in the chat.

    Source: https://core.telegram.org/bots/api#videochatended
    """

    duration: int
    """Video chat duration in seconds"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, duration: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(duration=duration, **__pydantic_kwargs)
