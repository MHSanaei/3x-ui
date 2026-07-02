from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import StoryAreaTypeType

from .story_area_type import StoryAreaType


class StoryAreaTypeLink(StoryAreaType):
    """
    Describes a story area pointing to an HTTP or tg:// link. Currently, a story can have up to 3 link areas.

    Source: https://core.telegram.org/bots/api#storyareatypelink
    """

    type: Literal[StoryAreaTypeType.LINK] = StoryAreaTypeType.LINK
    """Type of the area, always 'link'"""
    url: str
    """HTTP or tg:// URL to be opened when the area is clicked"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[StoryAreaTypeType.LINK] = StoryAreaTypeType.LINK,
            url: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, url=url, **__pydantic_kwargs)
