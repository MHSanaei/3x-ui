from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class GiftBackground(TelegramObject):
    """
    This object describes the background of a gift.

    Source: https://core.telegram.org/bots/api#giftbackground
    """

    center_color: int
    """Center color of the background in RGB format"""
    edge_color: int
    """Edge color of the background in RGB format"""
    text_color: int
    """Text color of the background in RGB format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            center_color: int,
            edge_color: int,
            text_color: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                center_color=center_color,
                edge_color=edge_color,
                text_color=text_color,
                **__pydantic_kwargs,
            )
