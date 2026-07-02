from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from .background_type import BackgroundType

if TYPE_CHECKING:
    from .document import Document


class BackgroundTypeWallpaper(BackgroundType):
    """
    The background is a wallpaper in the JPEG format.

    Source: https://core.telegram.org/bots/api#backgroundtypewallpaper
    """

    type: Literal["wallpaper"] = "wallpaper"
    """Type of the background, always 'wallpaper'"""
    document: Document
    """Document with the wallpaper"""
    dark_theme_dimming: int
    """Dimming of the background in dark themes, as a percentage; 0-100"""
    is_blurred: bool | None = None
    """*Optional*. :code:`True`, if the wallpaper is downscaled to fit in a 450x450 square and then box-blurred with radius 12"""
    is_moving: bool | None = None
    """*Optional*. :code:`True`, if the background moves slightly when the device is tilted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["wallpaper"] = "wallpaper",
            document: Document,
            dark_theme_dimming: int,
            is_blurred: bool | None = None,
            is_moving: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                document=document,
                dark_theme_dimming=dark_theme_dimming,
                is_blurred=is_blurred,
                is_moving=is_moving,
                **__pydantic_kwargs,
            )
