from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetStickerSetTitle(TelegramMethod[bool]):
    """
    Use this method to set the title of a created sticker set. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickersettitle
    """

    __returning__ = bool
    __api_method__ = "setStickerSetTitle"

    name: str
    """Sticker set name"""
    title: str
    """Sticker set title, 1-64 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, name: str, title: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, title=title, **__pydantic_kwargs)
