from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class DeleteStickerSet(TelegramMethod[bool]):
    """
    Use this method to delete a sticker set that was created by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletestickerset
    """

    __returning__ = bool
    __api_method__ = "deleteStickerSet"

    name: str
    """Sticker set name"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, name: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(name=name, **__pydantic_kwargs)
