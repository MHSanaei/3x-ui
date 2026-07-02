from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .sticker import Sticker


class UniqueGiftModel(TelegramObject):
    """
    This object describes the model of a unique gift.

    Source: https://core.telegram.org/bots/api#uniquegiftmodel
    """

    name: str
    """Name of the model"""
    sticker: Sticker
    """The sticker that represents the unique gift"""
    rarity_per_mille: int
    """The number of unique gifts that receive this model for every 1000 gift upgrades. Always 0 for crafted gifts"""
    rarity: str | None = None
    """*Optional*. Rarity of the model if it is a crafted model. Currently, can be 'uncommon', 'rare', 'epic', or 'legendary'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str,
            sticker: Sticker,
            rarity_per_mille: int,
            rarity: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                name=name,
                sticker=sticker,
                rarity_per_mille=rarity_per_mille,
                rarity=rarity,
                **__pydantic_kwargs,
            )
