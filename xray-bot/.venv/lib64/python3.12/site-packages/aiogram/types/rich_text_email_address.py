from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextEmailAddress(RichText):
    """
    A text with an email address.

    Source: https://core.telegram.org/bots/api#richtextemailaddress
    """

    type: Literal[RichTextType.EMAIL_ADDRESS] = RichTextType.EMAIL_ADDRESS
    """Type of the rich text, always 'email_address'"""
    text: RichTextUnion
    """The text"""
    email_address: str
    """The email address"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.EMAIL_ADDRESS] = RichTextType.EMAIL_ADDRESS,
            text: RichTextUnion,
            email_address: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, text=text, email_address=email_address, **__pydantic_kwargs
            )
