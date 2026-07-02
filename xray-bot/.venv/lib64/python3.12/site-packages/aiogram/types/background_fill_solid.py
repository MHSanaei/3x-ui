from typing import TYPE_CHECKING, Any, Literal

from .background_fill import BackgroundFill


class BackgroundFillSolid(BackgroundFill):
    """
    The background is filled using the selected color.

    Source: https://core.telegram.org/bots/api#backgroundfillsolid
    """

    type: Literal["solid"] = "solid"
    """Type of the background fill, always 'solid'"""
    color: int
    """The color of the background fill in the RGB24 format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["solid"] = "solid",
            color: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, color=color, **__pydantic_kwargs)
