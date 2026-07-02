from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputProfilePhotoUnion
from .base import TelegramMethod


class SetMyProfilePhoto(TelegramMethod[bool]):
    """
    Changes the profile photo of the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmyprofilephoto
    """

    __returning__ = bool
    __api_method__ = "setMyProfilePhoto"

    photo: InputProfilePhotoUnion
    """The new profile photo to set"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, photo: InputProfilePhotoUnion, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(photo=photo, **__pydantic_kwargs)
