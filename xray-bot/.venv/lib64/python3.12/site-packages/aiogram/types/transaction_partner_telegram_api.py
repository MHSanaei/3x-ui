from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner


class TransactionPartnerTelegramApi(TransactionPartner):
    """
    Describes a transaction with payment for `paid broadcasting <https://core.telegram.org/bots/api#paid-broadcasts>`_.

    Source: https://core.telegram.org/bots/api#transactionpartnertelegramapi
    """

    type: Literal[TransactionPartnerType.TELEGRAM_API] = TransactionPartnerType.TELEGRAM_API
    """Type of the transaction partner, always 'telegram_api'"""
    request_count: int
    """The number of successful requests that exceeded regular limits and were therefore billed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                TransactionPartnerType.TELEGRAM_API
            ] = TransactionPartnerType.TELEGRAM_API,
            request_count: int,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, request_count=request_count, **__pydantic_kwargs)
