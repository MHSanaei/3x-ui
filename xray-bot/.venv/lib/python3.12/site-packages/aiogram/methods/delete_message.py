from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class DeleteMessage(TelegramMethod[bool]):
    """
    Use this method to delete a message, including service messages, with the following limitations:

    - A message can only be deleted if it was sent less than 48 hours ago.

    - Service messages about a supergroup, channel, or forum topic creation can't be deleted.

    - A dice message in a private chat can only be deleted if it was sent more than 24 hours ago.

    - Bots can delete outgoing messages in private chats, groups, and supergroups.

    - Bots can delete incoming messages in private chats.

    - Bots granted *can_post_messages* permissions can delete outgoing messages in channels.

    - If the bot is an administrator of a group, it can delete any message there.

    - If the bot has *can_delete_messages* administrator right in a supergroup or a channel, it can delete any message there.

    - If the bot has *can_manage_direct_messages* administrator right in a channel, it can delete any message in the corresponding direct messages chat.

    Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#deletemessage
    """

    __returning__ = bool
    __api_method__ = "deleteMessage"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_id: int
    """Identifier of the message to delete"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat_id: ChatIdUnion, message_id: int, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, message_id=message_id, **__pydantic_kwargs)
