from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .rich_block import RichBlock
    from .rich_block_union import RichBlockUnion


class RichBlockListItem(TelegramObject):
    """
    An item of a list.

    Source: https://core.telegram.org/bots/api#richblocklistitem
    """

    label: str
    """Label of the item"""
    blocks: list[RichBlockUnion]
    """The content of the item"""
    has_checkbox: bool | None = None
    """*Optional*. :code:`True`, if the item has a checkbox"""
    is_checked: bool | None = None
    """*Optional*. :code:`True`, if the item has a checked checkbox"""
    value: int | None = None
    """*Optional*. For ordered lists, the numeric value of the item label"""
    type: str | None = None
    """*Optional*. For ordered lists, the type of the item label; must be one of 'a' for lowercase letters, 'A' for uppercase letters, 'i' for lowercase Roman numerals, 'I' for uppercase Roman numerals, or '1' for decimal numbers"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            label: str,
            blocks: list[RichBlockUnion],
            has_checkbox: bool | None = None,
            is_checked: bool | None = None,
            value: int | None = None,
            type: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                label=label,
                blocks=blocks,
                has_checkbox=has_checkbox,
                is_checked=is_checked,
                value=value,
                type=type,
                **__pydantic_kwargs,
            )
