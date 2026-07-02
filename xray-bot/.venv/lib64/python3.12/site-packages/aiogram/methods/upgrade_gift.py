from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class UpgradeGift(TelegramMethod[bool]):
    """
    Upgrades a given regular gift to a unique gift. Requires the *can_transfer_and_upgrade_gifts* business bot right. Additionally requires the *can_transfer_stars* business bot right if the upgrade is paid. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#upgradegift
    """

    __returning__ = bool
    __api_method__ = "upgradeGift"

    business_connection_id: str
    """Unique identifier of the business connection"""
    owned_gift_id: str
    """Unique identifier of the regular gift that should be upgraded to a unique one"""
    keep_original_details: bool | None = None
    """Pass :code:`True` to keep the original gift text, sender and receiver in the upgraded gift"""
    star_count: int | None = None
    """The amount of Telegram Stars that will be paid for the upgrade from the business account balance. If :code:`gift.prepaid_upgrade_star_count > 0`, then pass 0, otherwise, the *can_transfer_stars* business bot right is required and :code:`gift.upgrade_star_count` must be passed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            owned_gift_id: str,
            keep_original_details: bool | None = None,
            star_count: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                owned_gift_id=owned_gift_id,
                keep_original_details=keep_original_details,
                star_count=star_count,
                **__pydantic_kwargs,
            )
