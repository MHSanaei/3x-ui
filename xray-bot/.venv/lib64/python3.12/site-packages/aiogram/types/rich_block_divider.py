from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock


class RichBlockDivider(RichBlock):
    """
    A divider, corresponding to the HTML tag :code:`<hr/>`.

    Source: https://core.telegram.org/bots/api#richblockdivider
    """

    type: Literal[RichBlockType.DIVIDER] = RichBlockType.DIVIDER
    """Type of the block, always 'divider'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.DIVIDER] = RichBlockType.DIVIDER,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
