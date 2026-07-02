from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner


class TransactionPartnerOther(TransactionPartner):
    """
    Describes a transaction with an unknown source or recipient.

    Source: https://core.telegram.org/bots/api#transactionpartnerother
    """

    type: Literal[TransactionPartnerType.OTHER] = TransactionPartnerType.OTHER
    """Type of the transaction partner, always 'other'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[TransactionPartnerType.OTHER] = TransactionPartnerType.OTHER,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
