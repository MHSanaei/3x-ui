from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputStoryContentUnion, MessageEntity, Story, StoryArea
from .base import TelegramMethod


class EditStory(TelegramMethod[Story]):
    """
    Edits a story previously posted by the bot on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :class:`aiogram.types.story.Story` on success.

    Source: https://core.telegram.org/bots/api#editstory
    """

    __returning__ = Story
    __api_method__ = "editStory"

    business_connection_id: str
    """Unique identifier of the business connection"""
    story_id: int
    """Unique identifier of the story to edit"""
    content: InputStoryContentUnion
    """Content of the story"""
    caption: str | None = None
    """Caption of the story, 0-2048 characters after entities parsing"""
    parse_mode: str | None = None
    """Mode for parsing entities in the story caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    areas: list[StoryArea] | None = None
    """A JSON-serialized list of clickable areas to be shown on the story"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            business_connection_id: str,
            story_id: int,
            content: InputStoryContentUnion,
            caption: str | None = None,
            parse_mode: str | None = None,
            caption_entities: list[MessageEntity] | None = None,
            areas: list[StoryArea] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                business_connection_id=business_connection_id,
                story_id=story_id,
                content=content,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                areas=areas,
                **__pydantic_kwargs,
            )
