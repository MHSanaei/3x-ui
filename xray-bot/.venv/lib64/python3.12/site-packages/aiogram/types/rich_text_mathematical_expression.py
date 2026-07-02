from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText


class RichTextMathematicalExpression(RichText):
    """
    A mathematical expression.

    Source: https://core.telegram.org/bots/api#richtextmathematicalexpression
    """

    type: Literal[RichTextType.MATHEMATICAL_EXPRESSION] = RichTextType.MATHEMATICAL_EXPRESSION
    """Type of the rich text, always 'mathematical_expression'"""
    expression: str
    """The expression in LaTeX format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                RichTextType.MATHEMATICAL_EXPRESSION
            ] = RichTextType.MATHEMATICAL_EXPRESSION,
            expression: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, expression=expression, **__pydantic_kwargs)
