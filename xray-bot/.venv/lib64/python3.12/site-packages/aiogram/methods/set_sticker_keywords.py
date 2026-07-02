from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetStickerKeywords(TelegramMethod[bool]):
    """
    Use this method to change search keywords assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickerkeywords
    """

    __returning__ = bool
    __api_method__ = "setStickerKeywords"

    sticker: str
    """File identifier of the sticker"""
    keywords: list[str] | None = None
    """A JSON-serialized list of 0-20 search keywords for the sticker with total length of up to 64 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            sticker: str,
            keywords: list[str] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(sticker=sticker, keywords=keywords, **__pydantic_kwargs)
