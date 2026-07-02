from __future__ import annotations

from typing import TypeAlias

from .revenue_withdrawal_state_failed import RevenueWithdrawalStateFailed
from .revenue_withdrawal_state_pending import RevenueWithdrawalStatePending
from .revenue_withdrawal_state_succeeded import RevenueWithdrawalStateSucceeded

RevenueWithdrawalStateUnion: TypeAlias = (
    RevenueWithdrawalStatePending | RevenueWithdrawalStateSucceeded | RevenueWithdrawalStateFailed
)
