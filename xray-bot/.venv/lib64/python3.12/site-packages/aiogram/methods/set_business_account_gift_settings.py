from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import AcceptedGiftTypes
from .base import TelegramMethod


class SetBusinessAccountGiftSettings(TelegramMethod[bool]):
    """
    Changes the privacy settings pertaining to incoming gifts in a managed business account. Requires the *can_change_gift_settings* business bot right. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setbusinessaccountgiftsettings
    """

    __returning__ = bool
    __api_method__ = "setBusinessAccountGiftSettings"

    business_connection_id: str
    """Unique identifier of the business connection"""
    show_gift_button: bool
    """Pass :code:`True`, if a button for sending a gift to the user or by the business account must always be shown in the input field"""
    accepted_gift_types: AcceptedGiftTypes
    """Types of gifts accepted by the business account"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            show_gift_button: bool,
            accepted_gift_types: AcceptedGiftTypes,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                show_gift_button=show_gift_button,
                accepted_gift_types=accepted_gift_types,
                **__pydantic_kwargs,
            )
