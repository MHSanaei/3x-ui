from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockTableCell(TelegramObject):
    """
    Cell in a table.

    Source: https://core.telegram.org/bots/api#richblocktablecell
    """

    align: str
    """Horizontal cell content alignment. Currently, must be one of 'left', 'center', or 'right'"""
    valign: str
    """Vertical cell content alignment. Currently, must be one of 'top', 'middle', or 'bottom'"""
    text: RichTextUnion | None = None
    """*Optional*. Text in the cell. If omitted, then the cell is invisible"""
    is_header: bool | None = None
    """*Optional*. :code:`True`, if the cell is a header cell"""
    colspan: int | None = None
    """*Optional*. The number of columns the cell spans if it is bigger than 1"""
    rowspan: int | None = None
    """*Optional*. The number of rows the cell spans if it is bigger than 1"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            align: str,
            valign: str,
            text: RichTextUnion | None = None,
            is_header: bool | None = None,
            colspan: int | None = None,
            rowspan: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                align=align,
                valign=valign,
                text=text,
                is_header=is_header,
                colspan=colspan,
                rowspan=rowspan,
                **__pydantic_kwargs,
            )
