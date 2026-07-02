from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextReferenceLink(RichText):
    """
    A link to a reference.

    Source: https://core.telegram.org/bots/api#richtextreferencelink
    """

    type: Literal[RichTextType.REFERENCE_LINK] = RichTextType.REFERENCE_LINK
    """Type of the rich text, always 'reference_link'"""
    text: RichTextUnion
    """The link text"""
    reference_name: str
    """The name of the reference"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.REFERENCE_LINK] = RichTextType.REFERENCE_LINK,
            text: RichTextUnion,
            reference_name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, text=text, reference_name=reference_name, **__pydantic_kwargs
            )
