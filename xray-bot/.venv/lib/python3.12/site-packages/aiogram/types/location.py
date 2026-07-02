from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class Location(TelegramObject):
    """
    This object represents a point on the map.

    Source: https://core.telegram.org/bots/api#location
    """

    latitude: float
    """Latitude as defined by the sender"""
    longitude: float
    """Longitude as defined by the sender"""
    horizontal_accuracy: float | None = None
    """*Optional*. The radius of uncertainty for the location, measured in meters; 0-1500"""
    live_period: int | None = None
    """*Optional*. Time relative to the message sending date, during which the location can be updated; in seconds. For active live locations only"""
    heading: int | None = None
    """*Optional*. The direction in which user is moving, in degrees; 1-360. For active live locations only"""
    proximity_alert_radius: int | None = None
    """*Optional*. The maximum distance for proximity alerts about approaching another chat member, in meters. For sent live locations only"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            latitude: float,
            longitude: float,
            horizontal_accuracy: float | None = None,
            live_period: int | None = None,
            heading: int | None = None,
            proximity_alert_radius: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                latitude=latitude,
                longitude=longitude,
                horizontal_accuracy=horizontal_accuracy,
                live_period=live_period,
                heading=heading,
                proximity_alert_radius=proximity_alert_radius,
                **__pydantic_kwargs,
            )
