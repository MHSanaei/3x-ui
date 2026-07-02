from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockPullQuotation(RichBlock):
    """
    A quotation with centered text, loosely corresponding to the HTML tag :code:`<aside>`.

    Source: https://core.telegram.org/bots/api#richblockpullquotation
    """

    type: Literal[RichBlockType.PULLQUOTE] = RichBlockType.PULLQUOTE
    """Type of the block, always 'pullquote'"""
    text: RichTextUnion
    """Text of the block"""
    credit: RichTextUnion | None = None
    """*Optional*. Credit of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.PULLQUOTE] = RichBlockType.PULLQUOTE,
            text: RichTextUnion,
            credit: RichTextUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, credit=credit, **__pydantic_kwargs)
