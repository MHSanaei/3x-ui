from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatBoostSourceType
from .chat_boost_source import ChatBoostSource

if TYPE_CHECKING:
    from .user import User


class ChatBoostSourcePremium(ChatBoostSource):
    """
    The boost was obtained by subscribing to Telegram Premium or by gifting a Telegram Premium subscription to another user.

    Source: https://core.telegram.org/bots/api#chatboostsourcepremium
    """

    source: Literal[ChatBoostSourceType.PREMIUM] = ChatBoostSourceType.PREMIUM
    """Source of the boost, always 'premium'"""
    user: User
    """User that boosted the chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[ChatBoostSourceType.PREMIUM] = ChatBoostSourceType.PREMIUM,
            user: User,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(source=source, user=user, **__pydantic_kwargs)
