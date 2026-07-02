from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputSticker
from .base import TelegramMethod


class ReplaceStickerInSet(TelegramMethod[bool]):
    """
    Use this method to replace an existing sticker in a sticker set with a new one. The method is equivalent to calling :class:`aiogram.methods.delete_sticker_from_set.DeleteStickerFromSet`, then :class:`aiogram.methods.add_sticker_to_set.AddStickerToSet`, then :class:`aiogram.methods.set_sticker_position_in_set.SetStickerPositionInSet`. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#replacestickerinset
    """

    __returning__ = bool
    __api_method__ = "replaceStickerInSet"

    user_id: int
    """User identifier of the sticker set owner"""
    name: str
    """Sticker set name"""
    old_sticker: str
    """File identifier of the replaced sticker"""
    sticker: InputSticker
    """A JSON-serialized object with information about the added sticker. If exactly the same sticker had already been added to the set, then the set remains unchanged"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            name: str,
            old_sticker: str,
            sticker: InputSticker,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                name=name,
                old_sticker=old_sticker,
                sticker=sticker,
                **__pydantic_kwargs,
            )
