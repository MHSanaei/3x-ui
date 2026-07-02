from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..types import InputSticker
from .base import TelegramMethod


class CreateNewStickerSet(TelegramMethod[bool]):
    """
    Use this method to create a new sticker set owned by a user. The bot will be able to edit the sticker set thus created. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#createnewstickerset
    """

    __returning__ = bool
    __api_method__ = "createNewStickerSet"

    user_id: int
    """User identifier of created sticker set owner"""
    name: str
    """Short name of sticker set, to be used in :code:`t.me/addstickers/` URLs (e.g., *animals*). Can contain only English letters, digits and underscores. Must begin with a letter, can't contain consecutive underscores and must end in :code:`"_by_<bot_username>"`. :code:`<bot_username>` is case insensitive. 1-64 characters"""
    title: str
    """Sticker set title, 1-64 characters"""
    stickers: list[InputSticker]
    """A JSON-serialized list of 1-50 initial stickers to be added to the sticker set"""
    sticker_type: str | None = None
    """Type of stickers in the set, pass 'regular', 'mask', or 'custom_emoji'. By default, a regular sticker set is created"""
    needs_repainting: bool | None = None
    """Pass :code:`True` if stickers in the sticker set must be repainted to the color of text when used in messages, the accent color if used as emoji status, white on chat photos, or another appropriate color based on context; for custom emoji sticker sets only"""
    sticker_format: str | None = Field(None, json_schema_extra={"deprecated": True})
    """Format of stickers in the set, must be one of 'static', 'animated', 'video'

.. deprecated:: API:7.2
   https://core.telegram.org/bots/api-changelog#march-31-2024"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            name: str,
            title: str,
            stickers: list[InputSticker],
            sticker_type: str | None = None,
            needs_repainting: bool | None = None,
            sticker_format: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                name=name,
                title=title,
                stickers=stickers,
                sticker_type=sticker_type,
                needs_repainting=needs_repainting,
                sticker_format=sticker_format,
                **__pydantic_kwargs,
            )
