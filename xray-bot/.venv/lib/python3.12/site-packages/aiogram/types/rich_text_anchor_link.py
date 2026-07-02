from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextAnchorLink(RichText):
    """
    A link to an anchor.

    Source: https://core.telegram.org/bots/api#richtextanchorlink
    """

    type: Literal[RichTextType.ANCHOR_LINK] = RichTextType.ANCHOR_LINK
    """Type of the rich text, always 'anchor_link'"""
    text: RichTextUnion
    """The link text"""
    anchor_name: str
    """The name of the anchor. If the name is empty, then the link brings back to the top of the message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.ANCHOR_LINK] = RichTextType.ANCHOR_LINK,
            text: RichTextUnion,
            anchor_name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, anchor_name=anchor_name, **__pydantic_kwargs)
