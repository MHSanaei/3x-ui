from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import StarTransactions
from .base import TelegramMethod


class GetStarTransactions(TelegramMethod[StarTransactions]):
    """
    Returns the bot's Telegram Star transactions in chronological order. On success, returns a :class:`aiogram.types.star_transactions.StarTransactions` object.

    Source: https://core.telegram.org/bots/api#getstartransactions
    """

    __returning__ = StarTransactions
    __api_method__ = "getStarTransactions"

    offset: int | None = None
    """Number of transactions to skip in the response"""
    limit: int | None = None
    """The maximum number of transactions to be retrieved. Values between 1-100 are accepted. Defaults to 100"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            offset: int | None = None,
            limit: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(offset=offset, limit=limit, **__pydantic_kwargs)
