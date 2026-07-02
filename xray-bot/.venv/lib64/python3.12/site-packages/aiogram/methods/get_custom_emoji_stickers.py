from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import Sticker
from .base import TelegramMethod


class GetCustomEmojiStickers(TelegramMethod[list[Sticker]]):
    """
    Use this method to get information about custom emoji stickers by their identifiers. Returns an Array of :class:`aiogram.types.sticker.Sticker` objects.

    Source: https://core.telegram.org/bots/api#getcustomemojistickers
    """

    __returning__ = list[Sticker]
    __api_method__ = "getCustomEmojiStickers"

    custom_emoji_ids: list[str]
    """A JSON-serialized list of custom emoji identifiers. At most 200 custom emoji identifiers can be specified"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, custom_emoji_ids: list[str], **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(custom_emoji_ids=custom_emoji_ids, **__pydantic_kwargs)
