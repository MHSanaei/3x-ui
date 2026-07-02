from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetStickerPositionInSet(TelegramMethod[bool]):
    """
    Use this method to move a sticker in a set created by the bot to a specific position. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickerpositioninset
    """

    __returning__ = bool
    __api_method__ = "setStickerPositionInSet"

    sticker: str
    """File identifier of the sticker"""
    position: int
    """New sticker position in the set, zero-based"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, sticker: str, position: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(sticker=sticker, position=position, **__pydantic_kwargs)
