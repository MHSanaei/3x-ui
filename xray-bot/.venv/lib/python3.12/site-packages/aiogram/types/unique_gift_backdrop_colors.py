from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class UniqueGiftBackdropColors(TelegramObject):
    """
    This object describes the colors of the backdrop of a unique gift.

    Source: https://core.telegram.org/bots/api#uniquegiftbackdropcolors
    """

    center_color: int
    """The color in the center of the backdrop in RGB format"""
    edge_color: int
    """The color on the edges of the backdrop in RGB format"""
    symbol_color: int
    """The color to be applied to the symbol in RGB format"""
    text_color: int
    """The color for the text on the backdrop in RGB format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            center_color: int,
            edge_color: int,
            symbol_color: int,
            text_color: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                center_color=center_color,
                edge_color=edge_color,
                symbol_color=symbol_color,
                text_color=text_color,
                **__pydantic_kwargs,
            )
