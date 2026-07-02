from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .star_transaction import StarTransaction


class StarTransactions(TelegramObject):
    """
    Contains a list of Telegram Star transactions.

    Source: https://core.telegram.org/bots/api#startransactions
    """

    transactions: list[StarTransaction]
    """The list of transactions"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, transactions: list[StarTransaction], **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(transactions=transactions, **__pydantic_kwargs)
