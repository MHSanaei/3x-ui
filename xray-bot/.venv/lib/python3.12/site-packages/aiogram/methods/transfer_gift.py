from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class TransferGift(TelegramMethod[bool]):
    """
    Transfers an owned unique gift to another user. Requires the *can_transfer_and_upgrade_gifts* business bot right. Requires *can_transfer_stars* business bot right if the transfer is paid. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#transfergift
    """

    __returning__ = bool
    __api_method__ = "transferGift"

    business_connection_id: str
    """Unique identifier of the business connection"""
    owned_gift_id: str
    """Unique identifier of the regular gift that should be transferred"""
    new_owner_chat_id: int
    """Unique identifier of the chat which will own the gift. The chat must be active in the last 24 hours"""
    star_count: int | None = None
    """The amount of Telegram Stars that will be paid for the transfer from the business account balance. If positive, then the *can_transfer_stars* business bot right is required"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            owned_gift_id: str,
            new_owner_chat_id: int,
            star_count: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                owned_gift_id=owned_gift_id,
                new_owner_chat_id=new_owner_chat_id,
                star_count=star_count,
                **__pydantic_kwargs,
            )
