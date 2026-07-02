from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class DeleteAllMessageReactions(TelegramMethod[bool]):
    """
    Use this method to remove up to 10000 recent reactions in a group or a supergroup chat added by a given user or chat. The bot must have the 'can_delete_messages' administrator right in the chat. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deleteallmessagereactions
    """

    __returning__ = bool
    __api_method__ = "deleteAllMessageReactions"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    user_id: int | None = None
    """Identifier of the user whose reactions will be removed, if the reactions were added by a user"""
    actor_chat_id: int | None = None
    """Identifier of the chat whose reactions will be removed, if the reactions were added by a chat"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int | None = None,
            actor_chat_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, user_id=user_id, actor_chat_id=actor_chat_id, **__pydantic_kwargs
            )
