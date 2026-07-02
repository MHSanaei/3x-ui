from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class BusinessOpeningHoursInterval(TelegramObject):
    """
    Describes an interval of time during which a business is open.

    Source: https://core.telegram.org/bots/api#businessopeninghoursinterval
    """

    opening_minute: int
    """The minute's sequence number in a week, starting on Monday, marking the start of the time interval during which the business is open; 0 - 7 * 24 * 60"""
    closing_minute: int
    """The minute's sequence number in a week, starting on Monday, marking the end of the time interval during which the business is open; 0 - 8 * 24 * 60"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            opening_minute: int,
            closing_minute: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                opening_minute=opening_minute, closing_minute=closing_minute, **__pydantic_kwargs
            )
