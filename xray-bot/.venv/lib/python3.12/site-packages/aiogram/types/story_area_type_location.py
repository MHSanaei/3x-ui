from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import StoryAreaTypeType

from .story_area_type import StoryAreaType

if TYPE_CHECKING:
    from .location_address import LocationAddress


class StoryAreaTypeLocation(StoryAreaType):
    """
    Describes a story area pointing to a location. Currently, a story can have up to 10 location areas.

    Source: https://core.telegram.org/bots/api#storyareatypelocation
    """

    type: Literal[StoryAreaTypeType.LOCATION] = StoryAreaTypeType.LOCATION
    """Type of the area, always 'location'"""
    latitude: float
    """Location latitude in degrees"""
    longitude: float
    """Location longitude in degrees"""
    address: LocationAddress | None = None
    """*Optional*. Address of the location"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[StoryAreaTypeType.LOCATION] = StoryAreaTypeType.LOCATION,
            latitude: float,
            longitude: float,
            address: LocationAddress | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                latitude=latitude,
                longitude=longitude,
                address=address,
                **__pydantic_kwargs,
            )
