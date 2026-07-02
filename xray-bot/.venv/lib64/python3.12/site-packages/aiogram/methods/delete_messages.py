from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class DeleteMessages(TelegramMethod[bool]):
    """
    Use this method to delete multiple messages simultaneously. If some of the specified messages can't be found, they are skipped. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletemessages
    """

    __returning__ = bool
    __api_method__ = "deleteMessages"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_ids: list[int]
    """A JSON-serialized list of 1-100 identifiers of messages to delete. See :class:`aiogram.methods.delete_message.DeleteMessage` for limitations on which messages can be deleted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_ids: list[int],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, message_ids=message_ids, **__pydantic_kwargs)
