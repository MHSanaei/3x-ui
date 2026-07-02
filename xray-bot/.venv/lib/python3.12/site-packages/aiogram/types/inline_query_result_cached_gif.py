from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion
    from .message_entity import MessageEntity


class InlineQueryResultCachedGif(InlineQueryResult):
    """
    Represents a link to an animated GIF file stored on the Telegram servers. By default, this animated GIF file will be sent by the user with an optional caption. Alternatively, you can use *input_message_content* to send a message with specified content instead of the animation.

    Source: https://core.telegram.org/bots/api#inlinequeryresultcachedgif
    """

    type: Literal[InlineQueryResultType.GIF] = InlineQueryResultType.GIF
    """Type of the result, must be *gif*"""
    id: str
    """Unique identifier for this result, 1-64 bytes"""
    gif_file_id: str
    """A valid file identifier for the GIF file"""
    title: str | None = None
    """*Optional*. Title for the result"""
    caption: str | None = None
    """*Optional*. Caption of the GIF file to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | Default | None = Default("show_caption_above_media")
    """*Optional*. Pass :code:`True`, if the caption must be shown above the message media"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the GIF animation"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.GIF] = InlineQueryResultType.GIF,
            id: str,
            gif_file_id: str,
            title: str | None = None,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
            reply_markup: InlineKeyboardMarkup | None = None,
            input_message_content: InputMessageContentUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                id=id,
                gif_file_id=gif_file_id,
                title=title,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                **__pydantic_kwargs,
            )
