from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class InputRichMessage(TelegramObject):
    """
    Describes a rich message to be sent. Exactly **one** of the fields *html* or *markdown* must be used.

    Source: https://core.telegram.org/bots/api#inputrichmessage
    """

    html: str | None = None
    """*Optional*. Content of the rich message to send described using HTML formatting. See `rich message formatting options <https://core.telegram.org/bots/api#rich-message-formatting-options>`_ for more details"""
    markdown: str | None = None
    """*Optional*. Content of the rich message to send described using Markdown formatting. See `rich message formatting options <https://core.telegram.org/bots/api#rich-message-formatting-options>`_ for more details"""
    is_rtl: bool | None = None
    """*Optional*. Pass :code:`True` if the rich message must be shown right-to-left"""
    skip_entity_detection: bool | None = None
    """*Optional*. Pass :code:`True` to skip automatic detection of entities (e.g., URLs, email addresses, username mentions, hashtags, cashtags, bot commands, or phone numbers) in the text"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            html: str | None = None,
            markdown: str | None = None,
            is_rtl: bool | None = None,
            skip_entity_detection: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                html=html,
                markdown=markdown,
                is_rtl=is_rtl,
                skip_entity_detection=skip_entity_detection,
                **__pydantic_kwargs,
            )
