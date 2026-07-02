from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from .base import TelegramMethod


class EditForumTopic(TelegramMethod[bool]):
    """
    Use this method to edit name and icon of a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights, unless it is the creator of the topic. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#editforumtopic
    """

    __returning__ = bool
    __api_method__ = "editForumTopic"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    message_thread_id: int
    """Unique identifier for the target message thread of the forum topic"""
    name: str | None = None
    """New topic name, 0-128 characters. If not specified or empty, the current name of the topic will be kept"""
    icon_custom_emoji_id: str | None = None
    """New unique identifier of the custom emoji shown as the topic icon. Use :class:`aiogram.methods.get_forum_topic_icon_stickers.GetForumTopicIconStickers` to get all allowed custom emoji identifiers. Pass an empty string to remove the icon. If not specified, the current icon will be kept"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_thread_id: int,
            name: str | None = None,
            icon_custom_emoji_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                message_thread_id=message_thread_id,
                name=name,
                icon_custom_emoji_id=icon_custom_emoji_id,
                **__pydantic_kwargs,
            )
