from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class ChatBoostAdded(TelegramObject):
    """
    This object represents a service message about a user boosting a chat.

    Source: https://core.telegram.org/bots/api#chatboostadded
    """

    boost_count: int
    """Number of boosts added by the user"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, boost_count: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(boost_count=boost_count, **__pydantic_kwargs)
