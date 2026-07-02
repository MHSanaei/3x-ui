from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject


class ForumTopicEdited(TelegramObject):
    """
    This object represents a service message about an edited forum topic.

    Source: https://core.telegram.org/bots/api#forumtopicedited
    """

    name: str | None = None
    """*Optional*. New name of the topic, if it was edited"""
    icon_custom_emoji_id: str | None = None
    """*Optional*. New identifier of the custom emoji shown as the topic icon, if it was edited; an empty string if the icon was removed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            name: str | None = None,
            icon_custom_emoji_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                name=name, icon_custom_emoji_id=icon_custom_emoji_id, **__pydantic_kwargs
            )
