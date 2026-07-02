from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import MaskPosition
from .base import TelegramMethod


class SetStickerMaskPosition(TelegramMethod[bool]):
    """
    Use this method to change the `mask position <https://core.telegram.org/bots/api#maskposition>`_ of a mask sticker. The sticker must belong to a sticker set that was created by the bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setstickermaskposition
    """

    __returning__ = bool
    __api_method__ = "setStickerMaskPosition"

    sticker: str
    """File identifier of the sticker"""
    mask_position: MaskPosition | None = None
    """A JSON-serialized object with the position where the mask should be placed on faces. Omit the parameter to remove the mask position"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            sticker: str,
            mask_position: MaskPosition | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(sticker=sticker, mask_position=mask_position, **__pydantic_kwargs)
