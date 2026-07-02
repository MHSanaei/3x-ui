from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextUrl(RichText):
    """
    A text with a link.

    Source: https://core.telegram.org/bots/api#richtexturl
    """

    type: Literal[RichTextType.URL] = RichTextType.URL
    """Type of the rich text, always 'url'"""
    text: RichTextUnion
    """The text"""
    url: str
    """URL of the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.URL] = RichTextType.URL,
            text: RichTextUnion,
            url: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, url=url, **__pydantic_kwargs)
