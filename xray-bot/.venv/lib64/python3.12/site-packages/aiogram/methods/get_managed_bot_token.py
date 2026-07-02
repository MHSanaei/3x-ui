from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class GetManagedBotToken(TelegramMethod[str]):
    """
    Use this method to get the token of a managed bot. Returns the token as *String* on success.

    Source: https://core.telegram.org/bots/api#getmanagedbottoken
    """

    __returning__ = str
    __api_method__ = "getManagedBotToken"

    user_id: int
    """User identifier of the managed bot whose token will be returned"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, user_id: int, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, **__pydantic_kwargs)
