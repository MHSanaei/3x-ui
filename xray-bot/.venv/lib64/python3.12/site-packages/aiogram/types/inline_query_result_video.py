from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..client.default import Default
from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion
    from .message_entity import MessageEntity


class InlineQueryResultVideo(InlineQueryResult):
    """
    Represents a link to a page containing an embedded video player or a video file. By default, this video file will be sent by the user with an optional caption. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the video.

     If an InlineQueryResultVideo message contains an embedded video (e.g., YouTube), you **must** replace its content using *input_message_content*.

    Source: https://core.telegram.org/bots/api#inlinequeryresultvideo
    """

    type: Literal[InlineQueryResultType.VIDEO] = InlineQueryResultType.VIDEO
    """Type of the result, must be *video*"""
    id: str
    """Unique identifier for this result, 1-64 bytes"""
    video_url: str
    """A valid URL for the embedded video player or video file"""
    mime_type: str
    """MIME type of the content of the video URL, 'text/html' or 'video/mp4'"""
    thumbnail_url: str
    """URL of the thumbnail (JPEG only) for the video"""
    title: str
    """Title for the result"""
    caption: str | None = None
    """*Optional*. Caption of the video to be sent, 0-1024 characters after entities parsing"""
    parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the caption, which can be specified instead of *parse_mode*"""
    show_caption_above_media: bool | Default | None = Default("show_caption_above_media")
    """*Optional*. Pass :code:`True`, if the caption must be shown above the message media"""
    video_width: int | None = None
    """*Optional*. Video width"""
    video_height: int | None = None
    """*Optional*. Video height"""
    video_duration: int | None = None
    """*Optional*. Video duration in seconds"""
    description: str | None = None
    """*Optional*. Short description of the result"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the video. This field is **required** if InlineQueryResultVideo is used to send an HTML-page as a result (e.g., a YouTube video)"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.VIDEO] = InlineQueryResultType.VIDEO,
            id: str,
            video_url: str,
            mime_type: str,
            thumbnail_url: str,
            title: str,
            caption: str | None = None,
            parse_mode: str | Default | None = Default("parse_mode"),
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
            video_width: int | None = None,
            video_height: int | None = None,
            video_duration: int | None = None,
            description: str | None = None,
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
                video_url=video_url,
                mime_type=mime_type,
                thumbnail_url=thumbnail_url,
                title=title,
                caption=caption,
                parse_mode=parse_mode,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                video_width=video_width,
                video_height=video_height,
                video_duration=video_duration,
                description=description,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                **__pydantic_kwargs,
            )
