from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from .background_type import BackgroundType

if TYPE_CHECKING:
    from .background_fill_union import BackgroundFillUnion
    from .document import Document


class BackgroundTypePattern(BackgroundType):
    """
    The background is a .PNG or .TGV (gzipped subset of SVG with MIME type 'application/x-tgwallpattern') pattern to be combined with the background fill chosen by the user.

    Source: https://core.telegram.org/bots/api#backgroundtypepattern
    """

    type: Literal["pattern"] = "pattern"
    """Type of the background, always 'pattern'"""
    document: Document
    """Document with the pattern"""
    fill: BackgroundFillUnion
    """The background fill that is combined with the pattern"""
    intensity: int
    """Intensity of the pattern when it is shown above the filled background; 0-100"""
    is_inverted: bool | None = None
    """*Optional*. :code:`True`, if the background fill must be applied only to the pattern itself. All other pixels are black in this case. For dark themes only"""
    is_moving: bool | None = None
    """*Optional*. :code:`True`, if the background moves slightly when the device is tilted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["pattern"] = "pattern",
            document: Document,
            fill: BackgroundFillUnion,
            intensity: int,
            is_inverted: bool | None = None,
            is_moving: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                document=document,
                fill=fill,
                intensity=intensity,
                is_inverted=is_inverted,
                is_moving=is_moving,
                **__pydantic_kwargs,
            )
