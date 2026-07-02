from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputStoryContentUnion, MessageEntity, Story, StoryArea
from .base import TelegramMethod


class PostStory(TelegramMethod[Story]):
    """
    Posts a story on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :class:`aiogram.types.story.Story` on success.

    Source: https://core.telegram.org/bots/api#poststory
    """

    __returning__ = Story
    __api_method__ = "postStory"

    business_connection_id: str
    """Unique identifier of the business connection"""
    content: InputStoryContentUnion
    """Content of the story"""
    active_period: int
    """Period after which the story is moved to the archive, in seconds; must be one of :code:`6 * 3600`, :code:`12 * 3600`, :code:`86400`, or :code:`2 * 86400`"""
    caption: str | None = None
    """Caption of the story, 0-2048 characters after entities parsing"""
    parse_mode: str | None = None
    """Mode for parsing entities in the story caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    areas: list[StoryArea] | None = None
    """A JSON-serialized list of clickable areas to be shown on the story"""
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
            content: InputStoryContentUnion,
            active_period: int,
            caption: str | None = None,
            parse_mode: str | None = None,
            caption_entities: list[MessageEntity] | None = None,
            areas: list[StoryArea] | None = None,
            post_to_chat_page: bool | None = None,
            protect_content: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                content=content,
                active_period=active_period,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                areas=areas,
                post_to_chat_page=post_to_chat_page,
                protect_content=protect_content,
                **__pydantic_kwargs,
            )
