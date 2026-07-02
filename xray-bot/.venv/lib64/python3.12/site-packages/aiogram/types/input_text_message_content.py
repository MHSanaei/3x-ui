from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from .input_message_content import InputMessageContent

if TYPE_CHECKING:
    from .link_preview_options import LinkPreviewOptions
    from .message_entity import MessageEntity


class InputTextMessageContent(InputMessageContent):
    """
    Represents the `content <https://core.telegram.org/bots/api#inputmessagecontent>`_ of a text message to be sent as the result of an inline query.

    Source: https://core.telegram.org/bots/api#inputtextmessagecontent
    """

    message_text: str
    """Text of the message to be sent, 1-4096 characters"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in message text, which can be specified instead of *parse_mode*"""
    link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview")
    """*Optional*. Link preview generation options for the message"""
    disable_web_page_preview: bool | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. Disables link previews for links in the sent message

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            message_text: str,
            parse_mode: str | Default | None = Default("parse_mode"),
            entities: list[MessageEntity] | None = None,
            link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
            disable_web_page_preview: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                message_text=message_text,
                parse_mode=parse_mode,
                entities=entities,
                link_preview_options=link_preview_options,
                disable_web_page_preview=disable_web_page_preview,
                **__pydantic_kwargs,
            )
