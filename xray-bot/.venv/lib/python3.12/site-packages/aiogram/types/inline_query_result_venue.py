from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion


class InlineQueryResultVenue(InlineQueryResult):
    """
    Represents a venue. By default, the venue will be sent by the user. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the venue.

    Source: https://core.telegram.org/bots/api#inlinequeryresultvenue
    """

    type: Literal[InlineQueryResultType.VENUE] = InlineQueryResultType.VENUE
    """Type of the result, must be *venue*"""
    id: str
    """Unique identifier for this result, 1-64 Bytes"""
    latitude: float
    """Latitude of the venue location in degrees"""
    longitude: float
    """Longitude of the venue location in degrees"""
    title: str
    """Title of the venue"""
    address: str
    """Address of the venue"""
    foursquare_id: str | None = None
    """*Optional*. Foursquare identifier of the venue if known"""
    foursquare_type: str | None = None
    """*Optional*. Foursquare type of the venue, if known. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)"""
    google_place_id: str | None = None
    """*Optional*. Google Places identifier of the venue"""
    google_place_type: str | None = None
    """*Optional*. Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the venue"""
    thumbnail_url: str | None = None
    """*Optional*. Url of the thumbnail for the result"""
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
            type: Literal[InlineQueryResultType.VENUE] = InlineQueryResultType.VENUE,
            id: str,
            latitude: float,
            longitude: float,
            title: str,
            address: str,
            foursquare_id: str | None = None,
            foursquare_type: str | None = None,
            google_place_id: str | None = None,
            google_place_type: str | None = None,
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
                latitude=latitude,
                longitude=longitude,
                title=title,
                address=address,
                foursquare_id=foursquare_id,
                foursquare_type=foursquare_type,
                google_place_id=google_place_id,
                google_place_type=google_place_type,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                thumbnail_url=thumbnail_url,
                thumbnail_width=thumbnail_width,
                thumbnail_height=thumbnail_height,
                **__pydantic_kwargs,
            )
