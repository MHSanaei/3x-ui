from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RevenueWithdrawalStateType
from .custom import DateTime
from .revenue_withdrawal_state import RevenueWithdrawalState


class RevenueWithdrawalStateSucceeded(RevenueWithdrawalState):
    """
    The withdrawal succeeded.

    Source: https://core.telegram.org/bots/api#revenuewithdrawalstatesucceeded
    """

    type: Literal[RevenueWithdrawalStateType.SUCCEEDED] = RevenueWithdrawalStateType.SUCCEEDED
    """Type of the state, always 'succeeded'"""
    date: DateTime
    """Date the withdrawal was completed in Unix time"""
    url: str
    """An HTTPS URL that can be used to see transaction details"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                RevenueWithdrawalStateType.SUCCEEDED
            ] = RevenueWithdrawalStateType.SUCCEEDED,
            date: DateTime,
            url: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, date=date, url=url, **__pydantic_kwargs)
