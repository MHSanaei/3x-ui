from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .paid_media_union import PaidMediaUnion


class PaidMediaInfo(TelegramObject):
    """
    Describes the paid media added to a message.

    Source: https://core.telegram.org/bots/api#paidmediainfo
    """

    star_count: int
    """The number of Telegram Stars that must be paid to buy access to the media"""
    paid_media: list[PaidMediaUnion]
    """Information about the paid media"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            star_count: int,
            paid_media: list[PaidMediaUnion],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(star_count=star_count, paid_media=paid_media, **__pydantic_kwargs)
