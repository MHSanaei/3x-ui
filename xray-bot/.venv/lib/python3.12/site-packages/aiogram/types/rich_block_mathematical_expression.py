from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock


class RichBlockMathematicalExpression(RichBlock):
    """
    A block with a mathematical expression in LaTeX format, corresponding to the custom HTML tag :code:`<tg-math-block>`.

    Source: https://core.telegram.org/bots/api#richblockmathematicalexpression
    """

    type: Literal[RichBlockType.MATHEMATICAL_EXPRESSION] = RichBlockType.MATHEMATICAL_EXPRESSION
    """Type of the block, always 'mathematical_expression'"""
    expression: str
    """The mathematical expression in LaTeX format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                RichBlockType.MATHEMATICAL_EXPRESSION
            ] = RichBlockType.MATHEMATICAL_EXPRESSION,
            expression: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, expression=expression, **__pydantic_kwargs)
