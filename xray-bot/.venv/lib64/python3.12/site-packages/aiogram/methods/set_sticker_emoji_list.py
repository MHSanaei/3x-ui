from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetStickerEmojiList(TelegramMethod[bool]):
    """
    Use this method to change the list of emoji assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickeremojilist
    """

    __returning__ = bool
    __api_method__ = "setStickerEmojiList"

    sticker: str
    """File identifier of the sticker"""
    emoji_list: list[str]
    """A JSON-serialized list of 1-20 emoji associated with the sticker"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, sticker: str, emoji_list: list[str], **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(sticker=sticker, emoji_list=emoji_list, **__pydantic_kwargs)
