from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .gift import Gift


class Gifts(TelegramObject):
    """
    This object represent a list of gifts.

    Source: https://core.telegram.org/bots/api#gifts
    """

    gifts: list[Gift]
    """The list of gifts"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, gifts: list[Gift], **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(gifts=gifts, **__pydantic_kwargs)
