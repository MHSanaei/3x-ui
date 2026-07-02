from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextHashtag(RichText):
    """
    A hashtag.

    Source: https://core.telegram.org/bots/api#richtexthashtag
    """

    type: Literal[RichTextType.HASHTAG] = RichTextType.HASHTAG
    """Type of the rich text, always 'hashtag'"""
    text: RichTextUnion
    """The text"""
    hashtag: str
    """The hashtag"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.HASHTAG] = RichTextType.HASHTAG,
            text: RichTextUnion,
            hashtag: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, hashtag=hashtag, **__pydantic_kwargs)
