from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatBoostSourceType
from .chat_boost_source import ChatBoostSource

if TYPE_CHECKING:
    from .user import User


class ChatBoostSourceGiftCode(ChatBoostSource):
    """
    The boost was obtained by the creation of Telegram Premium gift codes to boost a chat. Each such code boosts the chat 4 times for the duration of the corresponding Telegram Premium subscription.

    Source: https://core.telegram.org/bots/api#chatboostsourcegiftcode
    """

    source: Literal[ChatBoostSourceType.GIFT_CODE] = ChatBoostSourceType.GIFT_CODE
    """Source of the boost, always 'gift_code'"""
    user: User
    """User for which the gift code was created"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[ChatBoostSourceType.GIFT_CODE] = ChatBoostSourceType.GIFT_CODE,
            user: User,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(source=source, user=user, **__pydantic_kwargs)
