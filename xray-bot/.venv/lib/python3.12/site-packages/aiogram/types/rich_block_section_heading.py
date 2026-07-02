from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockSectionHeading(RichBlock):
    """
    A section heading, corresponding to the HTML tags :code:`<h1>`, :code:`<h2>`, :code:`<h3>`, :code:`<h4>`, :code:`<h5>`, or :code:`<h6>`.

    Source: https://core.telegram.org/bots/api#richblocksectionheading
    """

    type: Literal[RichBlockType.HEADING] = RichBlockType.HEADING
    """Type of the block, always 'heading'"""
    text: RichTextUnion
    """Text of the block"""
    size: int
    """Relative size of the text font; 1-6, 1 is the largest, 6 is the smallest"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.HEADING] = RichBlockType.HEADING,
            text: RichTextUnion,
            size: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, size=size, **__pydantic_kwargs)
