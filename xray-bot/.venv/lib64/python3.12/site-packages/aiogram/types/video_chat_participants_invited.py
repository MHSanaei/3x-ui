from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .user import User


class VideoChatParticipantsInvited(TelegramObject):
    """
    This object represents a service message about new members invited to a video chat.

    Source: https://core.telegram.org/bots/api#videochatparticipantsinvited
    """

    users: list[User]
    """New members that were invited to the video chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, users: list[User], **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(users=users, **__pydantic_kwargs)
