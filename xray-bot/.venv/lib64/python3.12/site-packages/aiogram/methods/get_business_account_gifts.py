from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..types import OwnedGifts
from .base import TelegramMethod


class GetBusinessAccountGifts(TelegramMethod[OwnedGifts]):
    """
    Returns the gifts received and owned by a managed business account. Requires the *can_view_gifts_and_stars* business bot right. Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.

    Source: https://core.telegram.org/bots/api#getbusinessaccountgifts
    """

    __returning__ = OwnedGifts
    __api_method__ = "getBusinessAccountGifts"

    business_connection_id: str
    """Unique identifier of the business connection"""
    exclude_unsaved: bool | None = None
    """Pass :code:`True` to exclude gifts that aren't saved to the account's profile page"""
    exclude_saved: bool | None = None
    """Pass :code:`True` to exclude gifts that are saved to the account's profile page"""
    exclude_unlimited: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased an unlimited number of times"""
    exclude_limited_upgradable: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can be upgraded to unique"""
    exclude_limited_non_upgradable: bool | None = None
    """Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can't be upgraded to unique"""
    exclude_unique: bool | None = None
    """Pass :code:`True` to exclude unique gifts"""
    exclude_from_blockchain: bool | None = None
    """Pass :code:`True` to exclude gifts that were assigned from the TON blockchain and can't be resold or transferred in Telegram"""
    sort_by_price: bool | None = None
    """Pass :code:`True` to sort results by gift price instead of send date. Sorting is applied before pagination"""
    offset: str | None = None
    """Offset of the first entry to return as received from the previous request; use empty string to get the first chunk of results"""
    limit: int | None = None
    """The maximum number of gifts to be returned; 1-100. Defaults to 100"""
    exclude_limited: bool | None = Field(None, json_schema_extra={"deprecated": True})
    """Pass :code:`True` to exclude gifts that can be purchased a limited number of times

.. deprecated:: API:9.3
   https://core.telegram.org/bots/api-changelog#december-31-2025"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            exclude_unsaved: bool | None = None,
            exclude_saved: bool | None = None,
            exclude_unlimited: bool | None = None,
            exclude_limited_upgradable: bool | None = None,
            exclude_limited_non_upgradable: bool | None = None,
            exclude_unique: bool | None = None,
            exclude_from_blockchain: bool | None = None,
            sort_by_price: bool | None = None,
            offset: str | None = None,
            limit: int | None = None,
            exclude_limited: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                exclude_unsaved=exclude_unsaved,
                exclude_saved=exclude_saved,
                exclude_unlimited=exclude_unlimited,
                exclude_limited_upgradable=exclude_limited_upgradable,
                exclude_limited_non_upgradable=exclude_limited_non_upgradable,
                exclude_unique=exclude_unique,
                exclude_from_blockchain=exclude_from_blockchain,
                sort_by_price=sort_by_price,
                offset=offset,
                limit=limit,
                exclude_limited=exclude_limited,
                **__pydantic_kwargs,
            )
