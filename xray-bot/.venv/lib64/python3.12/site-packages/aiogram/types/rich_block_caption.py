from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockCaption(TelegramObject):
    """
    Caption of a rich formatted block.

    Source: https://core.telegram.org/bots/api#richblockcaption
    """

    text: RichTextUnion
    """Block caption"""
    credit: RichTextUnion | None = None
    """*Optional*. Block credit which corresponds to the HTML tag <cite>"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: RichTextUnion,
            credit: RichTextUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(text=text, credit=credit, **__pydantic_kwargs)
