from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime


class VideoChatScheduled(TelegramObject):
    """
    This object represents a service message about a video chat scheduled in the chat.

    Source: https://core.telegram.org/bots/api#videochatscheduled
    """

    start_date: DateTime
    """Point in time (Unix timestamp) when the video chat is supposed to be started by a chat administrator"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, start_date: DateTime, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(start_date=start_date, **__pydantic_kwargs)
