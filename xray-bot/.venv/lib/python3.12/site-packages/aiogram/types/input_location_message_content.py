from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .input_message_content import InputMessageContent


class InputLocationMessageContent(InputMessageContent):
    """
    Represents the `content <https://core.telegram.org/bots/api#inputmessagecontent>`_ of a location message to be sent as the result of an inline query.

    Source: https://core.telegram.org/bots/api#inputlocationmessagecontent
    """

    latitude: float
    """Latitude of the location in degrees"""
    longitude: float
    """Longitude of the location in degrees"""
    horizontal_accuracy: float | None = None
    """*Optional*. The radius of uncertainty for the location, measured in meters; 0-1500"""
    live_period: int | None = None
    """*Optional*. Period in seconds during which the location can be updated, must be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely"""
    heading: int | None = None
    """*Optional*. For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified"""
    proximity_alert_radius: int | None = None
    """*Optional*. For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified"""

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
