from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText


class RichTextAnchor(RichText):
    """
    An anchor.

    Source: https://core.telegram.org/bots/api#richtextanchor
    """

    type: Literal[RichTextType.ANCHOR] = RichTextType.ANCHOR
    """Type of the rich text, always 'anchor'"""
    name: str
    """The name of the anchor"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.ANCHOR] = RichTextType.ANCHOR,
            name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, name=name, **__pydantic_kwargs)
