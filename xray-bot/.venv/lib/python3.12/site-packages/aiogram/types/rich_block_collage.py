from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_block import RichBlock
    from .rich_block_caption import RichBlockCaption
    from .rich_block_union import RichBlockUnion


class RichBlockCollage(RichBlock):
    """
    A collage, corresponding to the custom HTML tag :code:`<tg-collage>`.

    Source: https://core.telegram.org/bots/api#richblockcollage
    """

    type: Literal[RichBlockType.COLLAGE] = RichBlockType.COLLAGE
    """Type of the block, always 'collage'"""
    blocks: list[RichBlockUnion]
    """Elements of the collage"""
    caption: RichBlockCaption | None = None
    """*Optional*. Caption of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.COLLAGE] = RichBlockType.COLLAGE,
            blocks: list[RichBlockUnion],
            caption: RichBlockCaption | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, blocks=blocks, caption=caption, **__pydantic_kwargs)
