from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import StickerSet
from .base import TelegramMethod


class GetStickerSet(TelegramMethod[StickerSet]):
    """
    Use this method to get a sticker set. On success, a :class:`aiogram.types.sticker_set.StickerSet` object is returned.

    Source: https://core.telegram.org/bots/api#getstickerset
    """

    __returning__ = StickerSet
    __api_method__ = "getStickerSet"

    name: str
    """Name of the sticker set"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, name: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, **__pydantic_kwargs)
