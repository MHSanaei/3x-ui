from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class DeleteBusinessMessages(TelegramMethod[bool]):
    """
    Delete messages on behalf of a business account. Requires the *can_delete_sent_messages* business bot right to delete messages sent by the bot itself, or the *can_delete_all_messages* business bot right to delete any message. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletebusinessmessages
    """

    __returning__ = bool
    __api_method__ = "deleteBusinessMessages"

    business_connection_id: str
    """Unique identifier of the business connection on behalf of which to delete the messages"""
    message_ids: list[int]
    """A JSON-serialized list of 1-100 identifiers of messages to delete. All messages must be from the same chat. See :class:`aiogram.methods.delete_message.DeleteMessage` for limitations on which messages can be deleted"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            message_ids: list[int],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                message_ids=message_ids,
                **__pydantic_kwargs,
            )
