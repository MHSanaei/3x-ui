from typing import TYPE_CHECKING, Any, Literal

from .background_fill import BackgroundFill


class BackgroundFillFreeformGradient(BackgroundFill):
    """
    The background is a freeform gradient that rotates after every message in the chat.

    Source: https://core.telegram.org/bots/api#backgroundfillfreeformgradient
    """

    type: Literal["freeform_gradient"] = "freeform_gradient"
    """Type of the background fill, always 'freeform_gradient'"""
    colors: list[int]
    """A list of the 3 or 4 base colors that are used to generate the freeform gradient in the RGB24 format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["freeform_gradient"] = "freeform_gradient",
            colors: list[int],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, colors=colors, **__pydantic_kwargs)
