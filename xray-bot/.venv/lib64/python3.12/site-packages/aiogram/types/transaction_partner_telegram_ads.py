from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner


class TransactionPartnerTelegramAds(TransactionPartner):
    """
    Describes a withdrawal transaction to the Telegram Ads platform.

    Source: https://core.telegram.org/bots/api#transactionpartnertelegramads
    """

    type: Literal[TransactionPartnerType.TELEGRAM_ADS] = TransactionPartnerType.TELEGRAM_ADS
    """Type of the transaction partner, always 'telegram_ads'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                TransactionPartnerType.TELEGRAM_ADS
            ] = TransactionPartnerType.TELEGRAM_ADS,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
