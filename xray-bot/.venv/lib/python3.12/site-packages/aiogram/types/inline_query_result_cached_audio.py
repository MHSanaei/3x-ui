from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion
    from .message_entity import MessageEntity


class InlineQueryResultCachedAudio(InlineQueryResult):
    """
    Represents a link to an MP3 audio file stored on the Telegram servers. By default, this audio file will be sent by the user. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the audio.

    Source: https://core.telegram.org/bots/api#inlinequeryresultcachedaudio
    """

    type: Literal[InlineQueryResultType.AUDIO] = InlineQueryResultType.AUDIO
    """Type of the result, must be *audio*"""
    id: str
    """Unique identifier for this result, 1-64 bytes"""
    audio_file_id: str
    """A valid file identifier for the audio file"""
    caption: str | None = None
    """*Optional*. Caption, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the audio caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the audio"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.AUDIO] = InlineQueryResultType.AUDIO,
            id: str,
            audio_file_id: str,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
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
                audio_file_id=audio_file_id,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                **__pydantic_kwargs,
            )
