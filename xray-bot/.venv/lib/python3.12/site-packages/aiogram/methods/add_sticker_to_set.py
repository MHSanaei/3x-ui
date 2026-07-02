from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputSticker
from .base import TelegramMethod


class AddStickerToSet(TelegramMethod[bool]):
    """
    Use this method to add a new sticker to a set created by the bot. Emoji sticker sets can have up to 200 stickers. Other sticker sets can have up to 120 stickers. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#addstickertoset
    """

    __returning__ = bool
    __api_method__ = "addStickerToSet"

    user_id: int
    """User identifier of sticker set owner"""
    name: str
    """Sticker set name"""
    sticker: InputSticker
    """A JSON-serialized object with information about the added sticker. If exactly the same sticker had already been added to the set, then the set isn't changed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            name: str,
            sticker: InputSticker,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, name=name, sticker=sticker, **__pydantic_kwargs)
