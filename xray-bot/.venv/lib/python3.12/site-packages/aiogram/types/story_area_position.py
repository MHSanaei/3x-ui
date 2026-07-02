from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class StoryAreaPosition(TelegramObject):
    """
    Describes the position of a clickable area within a story.

    Source: https://core.telegram.org/bots/api#storyareaposition
    """

    x_percentage: float
    """The abscissa of the area's center, as a percentage of the media width"""
    y_percentage: float
    """The ordinate of the area's center, as a percentage of the media height"""
    width_percentage: float
    """The width of the area's rectangle, as a percentage of the media width"""
    height_percentage: float
    """The height of the area's rectangle, as a percentage of the media height"""
    rotation_angle: float
    """The clockwise rotation angle of the rectangle, in degrees; 0-360"""
    corner_radius_percentage: float
    """The radius of the rectangle corner rounding, as a percentage of the media width"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            x_percentage: float,
            y_percentage: float,
            width_percentage: float,
            height_percentage: float,
            rotation_angle: float,
            corner_radius_percentage: float,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                x_percentage=x_percentage,
                y_percentage=y_percentage,
                width_percentage=width_percentage,
                height_percentage=height_percentage,
                rotation_angle=rotation_angle,
                corner_radius_percentage=corner_radius_percentage,
                **__pydantic_kwargs,
            )
