from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class SetManagedBotAccessSettings(TelegramMethod[bool]):
    """
    Use this method to change the access settings of a managed bot. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmanagedbotaccesssettings
    """

    __returning__ = bool
    __api_method__ = "setManagedBotAccessSettings"

    user_id: int
    """User identifier of the managed bot whose access settings will be changed"""
    is_access_restricted: bool
    """Pass :code:`True`, if only selected users can access the bot. The bot's owner can always access it"""
    added_user_ids: list[int] | None = None
    """A JSON-serialized list of up to 10 identifiers of users who will have access to the bot in addition to its owner. Ignored if *is_access_restricted* is false"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            is_access_restricted: bool,
            added_user_ids: list[int] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                is_access_restricted=is_access_restricted,
                added_user_ids=added_user_ids,
                **__pydantic_kwargs,
            )
