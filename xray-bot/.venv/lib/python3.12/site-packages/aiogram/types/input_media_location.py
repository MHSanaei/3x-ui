from typing import TYPE_CHECKING, Any, Literal

from ..enums import InputMediaType
from .input_poll_media import InputPollMedia
from .input_poll_option_media import InputPollOptionMedia


class InputMediaLocation(InputPollMedia, InputPollOptionMedia):
    """
    Represents a location to be sent.

    Source: https://core.telegram.org/bots/api#inputmedialocation
    """

    type: Literal[InputMediaType.LOCATION] = InputMediaType.LOCATION
    """Type of the media, must be *location*"""
    latitude: float
    """Latitude of the location"""
    longitude: float
    """Longitude of the location"""
    horizontal_accuracy: float | None = None
    """*Optional*. The radius of uncertainty for the location, measured in meters; 0-1500"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InputMediaType.LOCATION] = InputMediaType.LOCATION,
            latitude: float,
            longitude: float,
            horizontal_accuracy: float | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                latitude=latitude,
                longitude=longitude,
                horizontal_accuracy=horizontal_accuracy,
                **__pydantic_kwargs,
            )
