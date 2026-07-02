from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .user import User


class AffiliateInfo(TelegramObject):
    """
    Contains information about the affiliate that received a commission via this transaction.

    Source: https://core.telegram.org/bots/api#affiliateinfo
    """

    commission_per_mille: int
    """The number of Telegram Stars received by the affiliate for each 1000 Telegram Stars received by the bot from referred users"""
    amount: int
    """Integer amount of Telegram Stars received by the affiliate from the transaction, rounded to 0; can be negative for refunds"""
    affiliate_user: User | None = None
    """*Optional*. The bot or the user that received an affiliate commission if it was received by a bot or a user"""
    affiliate_chat: Chat | None = None
    """*Optional*. The chat that received an affiliate commission if it was received by a chat"""
    nanostar_amount: int | None = None
    """*Optional*. The number of 1/1000000000 shares of Telegram Stars received by the affiliate; from -999999999 to 999999999; can be negative for refunds"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            commission_per_mille: int,
            amount: int,
            affiliate_user: User | None = None,
            affiliate_chat: Chat | None = None,
            nanostar_amount: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                commission_per_mille=commission_per_mille,
                amount=amount,
                affiliate_user=affiliate_user,
                affiliate_chat=affiliate_chat,
                nanostar_amount=nanostar_amount,
                **__pydantic_kwargs,
            )
