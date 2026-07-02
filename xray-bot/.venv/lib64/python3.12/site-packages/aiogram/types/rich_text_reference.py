from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextReference(RichText):
    """
    A reference.

    Source: https://core.telegram.org/bots/api#richtextreference
    """

    type: Literal[RichTextType.REFERENCE] = RichTextType.REFERENCE
    """Type of the rich text, always 'reference'"""
    text: RichTextUnion
    """Text of the reference"""
    name: str
    """The name of the reference"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.REFERENCE] = RichTextType.REFERENCE,
            text: RichTextUnion,
            name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, name=name, **__pydantic_kwargs)
