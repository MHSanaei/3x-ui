from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import OwnedGifts
from .base import TelegramMethod


class GetUserGifts(TelegramMethod[OwnedGifts]):
    """
    Returns the gifts owned and hosted by a user. Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.

    Source: https://core.telegram.org/bots/api#getusergifts
    """

    __returning__ = OwnedGifts
    __api_method__ = "getUserGifts"

    user_id: int
    """Unique identifier of the user"""
    exclude_unlimited: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased an unlimited number of times"""
    exclude_limited_upgradable: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can be upgraded to unique"""
    exclude_limited_non_upgradable: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can't be upgraded to unique"""
    exclude_from_blockchain: bool | None = None
    """Pass :code:`True` to exclude gifts that were assigned from the TON blockchain and can't be resold or transferred in Telegram"""
    exclude_unique: bool | None = None
    """Pass :code:`True` to exclude unique gifts"""
    sort_by_price: bool | None = None
    """Pass :code:`True` to sort results by gift price instead of send date. Sorting is applied before pagination"""
    offset: str | None = None
    """Offset of the first entry to return as received from the previous request; use an empty string to get the first chunk of results"""
    limit: int | None = None
    """The maximum number of gifts to be returned; 1-100. Defaults to 100"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            exclude_unlimited: bool | None = None,
            exclude_limited_upgradable: bool | None = None,
            exclude_limited_non_upgradable: bool | None = None,
            exclude_from_blockchain: bool | None = None,
            exclude_unique: bool | None = None,
            sort_by_price: bool | None = None,
            offset: str | None = None,
            limit: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                exclude_unlimited=exclude_unlimited,
                exclude_limited_upgradable=exclude_limited_upgradable,
                exclude_limited_non_upgradable=exclude_limited_non_upgradable,
                exclude_from_blockchain=exclude_from_blockchain,
                exclude_unique=exclude_unique,
                sort_by_price=sort_by_price,
                offset=offset,
                limit=limit,
                **__pydantic_kwargs,
            )
