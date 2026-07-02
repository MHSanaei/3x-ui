from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .location import Location


class Venue(TelegramObject):
    """
    This object represents a venue.

    Source: https://core.telegram.org/bots/api#venue
    """

    location: Location
    """Venue location. Can't be a live location"""
    title: str
    """Name of the venue"""
    address: str
    """Address of the venue"""
    foursquare_id: str | None = None
    """*Optional*. Foursquare identifier of the venue"""
    foursquare_type: str | None = None
    """*Optional*. Foursquare type of the venue. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)"""
    google_place_id: str | None = None
    """*Optional*. Google Places identifier of the venue"""
    google_place_type: str | None = None
    """*Optional*. Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            location: Location,
            title: str,
            address: str,
            foursquare_id: str | None = None,
            foursquare_type: str | None = None,
            google_place_id: str | None = None,
            google_place_type: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                location=location,
                title=title,
                address=address,
                foursquare_id=foursquare_id,
                foursquare_type=foursquare_type,
                google_place_id=google_place_id,
                google_place_type=google_place_type,
                **__pydantic_kwargs,
            )
