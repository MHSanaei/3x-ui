from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextBankCardNumber(RichText):
    """
    A text with a bank card number.

    Source: https://core.telegram.org/bots/api#richtextbankcardnumber
    """

    type: Literal[RichTextType.BANK_CARD_NUMBER] = RichTextType.BANK_CARD_NUMBER
    """Type of the rich text, always 'bank_card_number'"""
    text: RichTextUnion
    """The text"""
    bank_card_number: str
    """The bank card number"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.BANK_CARD_NUMBER] = RichTextType.BANK_CARD_NUMBER,
            text: RichTextUnion,
            bank_card_number: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, text=text, bank_card_number=bank_card_number, **__pydantic_kwargs
            )
