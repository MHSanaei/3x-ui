from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetCustomEmojiStickerSetThumbnail(TelegramMethod[bool]):
    """
    Use this method to set the thumbnail of a custom emoji sticker set. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setcustomemojistickersetthumbnail
    """

    __returning__ = bool
    __api_method__ = "setCustomEmojiStickerSetThumbnail"

    name: str
    """Sticker set name"""
    custom_emoji_id: str | None = None
    """Custom emoji identifier of a sticker from the sticker set; pass an empty string to drop the thumbnail and use the first sticker as the thumbnail"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str,
            custom_emoji_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, custom_emoji_id=custom_emoji_id, **__pydantic_kwargs)
