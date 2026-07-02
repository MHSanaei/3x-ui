from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class AcceptedGiftTypes(TelegramObject):
    """
    This object describes the types of gifts that can be gifted to a user or a chat.

    Source: https://core.telegram.org/bots/api#acceptedgifttypes
    """

    unlimited_gifts: bool
    """:code:`True`, if unlimited regular gifts are accepted"""
    limited_gifts: bool
    """:code:`True`, if limited regular gifts are accepted"""
    unique_gifts: bool
    """:code:`True`, if unique gifts or gifts that can be upgraded to unique for free are accepted"""
    premium_subscription: bool
    """:code:`True`, if a Telegram Premium subscription is accepted"""
    gifts_from_channels: bool
    """:code:`True`, if transfers of unique gifts from channels are accepted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            unlimited_gifts: bool,
            limited_gifts: bool,
            unique_gifts: bool,
            premium_subscription: bool,
            gifts_from_channels: bool,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                unlimited_gifts=unlimited_gifts,
                limited_gifts=limited_gifts,
                unique_gifts=unique_gifts,
                premium_subscription=premium_subscription,
                gifts_from_channels=gifts_from_channels,
                **__pydantic_kwargs,
            )
