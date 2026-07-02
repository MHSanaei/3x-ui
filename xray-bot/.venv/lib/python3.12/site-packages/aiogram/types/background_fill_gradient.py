from typing import TYPE_CHECKING, Any, Literal

from .background_fill import BackgroundFill


class BackgroundFillGradient(BackgroundFill):
    """
    The background is a gradient fill.

    Source: https://core.telegram.org/bots/api#backgroundfillgradient
    """

    type: Literal["gradient"] = "gradient"
    """Type of the background fill, always 'gradient'"""
    top_color: int
    """Top color of the gradient in the RGB24 format"""
    bottom_color: int
    """Bottom color of the gradient in the RGB24 format"""
    rotation_angle: int
    """Clockwise rotation angle of the background fill in degrees; 0-359"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["gradient"] = "gradient",
            top_color: int,
            bottom_color: int,
            rotation_angle: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                top_color=top_color,
                bottom_color=bottom_color,
                rotation_angle=rotation_angle,
                **__pydantic_kwargs,
            )
