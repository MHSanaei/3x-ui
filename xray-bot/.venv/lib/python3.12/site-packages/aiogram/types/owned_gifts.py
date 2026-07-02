from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .owned_gift_union import OwnedGiftUnion


class OwnedGifts(TelegramObject):
    """
    Contains the list of gifts received and owned by a user or a chat.

    Source: https://core.telegram.org/bots/api#ownedgifts
    """

    total_count: int
    """The total number of gifts owned by the user or the chat"""
    gifts: list[OwnedGiftUnion]
    """The list of gifts"""
    next_offset: str | None = None
    """*Optional*. Offset for the next request. If empty, then there are no more results"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            total_count: int,
            gifts: list[OwnedGiftUnion],
            next_offset: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                total_count=total_count, gifts=gifts, next_offset=next_offset, **__pydantic_kwargs
            )
