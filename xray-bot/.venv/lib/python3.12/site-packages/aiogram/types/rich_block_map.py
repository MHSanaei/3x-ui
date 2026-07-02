from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .location import Location
    from .rich_block_caption import RichBlockCaption


class RichBlockMap(RichBlock):
    """
    A block with a map, corresponding to the custom HTML tag :code:`<tg-map>`.

    Source: https://core.telegram.org/bots/api#richblockmap
    """

    type: Literal[RichBlockType.MAP] = RichBlockType.MAP
    """Type of the block, always 'map'"""
    location: Location
    """Location of the center of the map"""
    zoom: int
    """Map zoom level; 13-20"""
    width: int
    """Expected width of the map"""
    height: int
    """Expected height of the map"""
    caption: RichBlockCaption | None = None
    """*Optional*. Caption of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.MAP] = RichBlockType.MAP,
            location: Location,
            zoom: int,
            width: int,
            height: int,
            caption: RichBlockCaption | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                location=location,
                zoom=zoom,
                width=width,
                height=height,
                caption=caption,
                **__pydantic_kwargs,
            )
