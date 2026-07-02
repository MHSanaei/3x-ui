from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .rich_block import RichBlock
    from .rich_block_union import RichBlockUnion


class RichMessage(TelegramObject):
    """
    Rich formatted message.

    Source: https://core.telegram.org/bots/api#richmessage
    """

    blocks: list[RichBlockUnion]
    """Content of the message"""
    is_rtl: bool | None = None
    """*Optional*. :code:`True`, if the rich message must be shown right-to-left"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            blocks: list[RichBlockUnion],
            is_rtl: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(blocks=blocks, is_rtl=is_rtl, **__pydantic_kwargs)
