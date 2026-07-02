from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .chat_boost_source_union import ChatBoostSourceUnion


class ChatBoost(TelegramObject):
    """
    This object contains information about a chat boost.

    Source: https://core.telegram.org/bots/api#chatboost
    """

    boost_id: str
    """Unique identifier of the boost"""
    add_date: DateTime
    """Point in time (Unix timestamp) when the chat was boosted"""
    expiration_date: DateTime
    """Point in time (Unix timestamp) when the boost will automatically expire, unless the booster's Telegram Premium subscription is prolonged"""
    source: ChatBoostSourceUnion
    """Source of the added boost"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            boost_id: str,
            add_date: DateTime,
            expiration_date: DateTime,
            source: ChatBoostSourceUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                boost_id=boost_id,
                add_date=add_date,
                expiration_date=expiration_date,
                source=source,
                **__pydantic_kwargs,
            )
