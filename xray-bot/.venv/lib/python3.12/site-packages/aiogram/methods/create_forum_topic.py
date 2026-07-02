from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ForumTopic
from .base import TelegramMethod


class CreateForumTopic(TelegramMethod[ForumTopic]):
    """
    Use this method to create a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator right. Returns information about the created topic as a :class:`aiogram.types.forum_topic.ForumTopic` object.

    Source: https://core.telegram.org/bots/api#createforumtopic
    """

    __returning__ = ForumTopic
    __api_method__ = "createForumTopic"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    name: str
    """Topic name, 1-128 characters"""
    icon_color: int | None = None
    """Color of the topic icon in RGB format. Currently, must be one of 7322096 (0x6FB9F0), 16766590 (0xFFD67E), 13338331 (0xCB86DB), 9367192 (0x8EEE98), 16749490 (0xFF93B2), or 16478047 (0xFB6F5F)"""
    icon_custom_emoji_id: str | None = None
    """Unique identifier of the custom emoji shown as the topic icon. Use :class:`aiogram.methods.get_forum_topic_icon_stickers.GetForumTopicIconStickers` to get all allowed custom emoji identifiers"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            name: str,
            icon_color: int | None = None,
            icon_custom_emoji_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                name=name,
                icon_color=icon_color,
                icon_custom_emoji_id=icon_custom_emoji_id,
                **__pydantic_kwargs,
            )
