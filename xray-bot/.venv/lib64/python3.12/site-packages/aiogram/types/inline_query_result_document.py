from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion
    from .message_entity import MessageEntity


class InlineQueryResultDocument(InlineQueryResult):
    """
    Represents a link to a file. By default, this file will be sent by the user with an optional caption. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the file. Currently, only **.PDF** and **.ZIP** files can be sent using this method.

    Source: https://core.telegram.org/bots/api#inlinequeryresultdocument
    """

    type: Literal[InlineQueryResultType.DOCUMENT] = InlineQueryResultType.DOCUMENT
    """Type of the result, must be *document*"""
    id: str
    """Unique identifier for this result, 1-64 bytes"""
    title: str
    """Title for the result"""
    document_url: str
    """A valid URL for the file"""
    mime_type: str
    """MIME type of the content of the file, either 'application/pdf' or 'application/zip'"""
    caption: str | None = None
    """*Optional*. Caption of the document to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    description: str | None = None
    """*Optional*. Short description of the result"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the file"""
    thumbnail_url: str | None = None
    """*Optional*. URL of the thumbnail (JPEG only) for the file"""
    thumbnail_width: int | None = None
    """*Optional*. Thumbnail width"""
    thumbnail_height: int | None = None
    """*Optional*. Thumbnail height"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.DOCUMENT] = InlineQueryResultType.DOCUMENT,
            id: str,
            title: str,
            document_url: str,
            mime_type: str,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            description: str | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            input_message_content: InputMessageContentUnion | None = None,
            thumbnail_url: str | None = None,
            thumbnail_width: int | None = None,
            thumbnail_height: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                id=id,
                title=title,
                document_url=document_url,
                mime_type=mime_type,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                description=description,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                thumbnail_url=thumbnail_url,
                thumbnail_width=thumbnail_width,
                thumbnail_height=thumbnail_height,
                **__pydantic_kwargs,
            )
