from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import Story
from .base import TelegramMethod


class RepostStory(TelegramMethod[Story]):
    """
    Reposts a story on behalf of a business account from another business account. Both business accounts must be managed by the same bot, and the story on the source account must have been posted (or reposted) by the bot. Requires the *can_manage_stories* business bot right for both business accounts. Returns :class:`aiogram.types.story.Story` on success.

    Source: https://core.telegram.org/bots/api#repoststory
    """

    __returning__ = Story
    __api_method__ = "repostStory"

    business_connection_id: str
    """Unique identifier of the business connection"""
    from_chat_id: int
    """Unique identifier of the chat which posted the story that should be reposted"""
    from_story_id: int
    """Unique identifier of the story that should be reposted"""
    active_period: int
    """Period after which the story is moved to the archive, in seconds; must be one of :code:`6 * 3600`, :code:`12 * 3600`, :code:`86400`, or :code:`2 * 86400`"""
    post_to_chat_page: bool | None = None
    """Pass :code:`True` to keep the story accessible after it expires"""
    protect_content: bool | None = None
    """Pass :code:`True` if the content of the story must be protected from forwarding and screenshotting"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            from_chat_id: int,
            from_story_id: int,
            active_period: int,
            post_to_chat_page: bool | None = None,
            protect_content: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                from_chat_id=from_chat_id,
                from_story_id=from_story_id,
                active_period=active_period,
                post_to_chat_page=post_to_chat_page,
                protect_content=protect_content,
                **__pydantic_kwargs,
            )
