from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_block import RichBlock
    from .rich_block_union import RichBlockUnion
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockDetails(RichBlock):
    """
    An expandable block for details disclosure, corresponding to the HTML tag :code:`<details>`.

    Source: https://core.telegram.org/bots/api#richblockdetails
    """

    type: Literal[RichBlockType.DETAILS] = RichBlockType.DETAILS
    """Type of the block, always 'details'"""
    summary: RichTextUnion
    """Always shown summary of the block"""
    blocks: list[RichBlockUnion]
    """Content of the block"""
    is_open: bool | None = None
    """*Optional*. :code:`True`, if the content of the block is visible by default"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.DETAILS] = RichBlockType.DETAILS,
            summary: RichTextUnion,
            blocks: list[RichBlockUnion],
            is_open: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, summary=summary, blocks=blocks, is_open=is_open, **__pydantic_kwargs
            )
