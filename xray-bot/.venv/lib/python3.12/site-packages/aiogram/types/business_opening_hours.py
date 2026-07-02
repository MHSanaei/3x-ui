from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .business_opening_hours_interval import BusinessOpeningHoursInterval


class BusinessOpeningHours(TelegramObject):
    """
    Describes the opening hours of a business.

    Source: https://core.telegram.org/bots/api#businessopeninghours
    """

    time_zone_name: str
    """Unique name of the time zone for which the opening hours are defined"""
    opening_hours: list[BusinessOpeningHoursInterval]
    """List of time intervals describing business opening hours"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            time_zone_name: str,
            opening_hours: list[BusinessOpeningHoursInterval],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                time_zone_name=time_zone_name, opening_hours=opening_hours, **__pydantic_kwargs
            )
