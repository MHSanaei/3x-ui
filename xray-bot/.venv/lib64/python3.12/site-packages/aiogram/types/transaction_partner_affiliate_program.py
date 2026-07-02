from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner

if TYPE_CHECKING:
    from .user import User


class TransactionPartnerAffiliateProgram(TransactionPartner):
    """
    Describes the affiliate program that issued the affiliate commission received via this transaction.

    Source: https://core.telegram.org/bots/api#transactionpartneraffiliateprogram
    """

    type: Literal[TransactionPartnerType.AFFILIATE_PROGRAM] = (
        TransactionPartnerType.AFFILIATE_PROGRAM
    )
    """Type of the transaction partner, always 'affiliate_program'"""
    commission_per_mille: int
    """The number of Telegram Stars received by the bot for each 1000 Telegram Stars received by the affiliate program sponsor from referred users"""
    sponsor_user: User | None = None
    """*Optional*. Information about the bot that sponsored the affiliate program"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                TransactionPartnerType.AFFILIATE_PROGRAM
            ] = TransactionPartnerType.AFFILIATE_PROGRAM,
            commission_per_mille: int,
            sponsor_user: User | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                commission_per_mille=commission_per_mille,
                sponsor_user=sponsor_user,
                **__pydantic_kwargs,
            )
