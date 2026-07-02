from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .animation import Animation
    from .audio import Audio
    from .document import Document
    from .link import Link
    from .live_photo import LivePhoto
    from .location import Location
    from .photo_size import PhotoSize
    from .sticker import Sticker
    from .venue import Venue
    from .video import Video


class PollMedia(TelegramObject):
    """
    At most **one** of the optional fields can be present in any given object.

    Source: https://core.telegram.org/bots/api#pollmedia
    """

    animation: Animation | None = None
    """*Optional*. Media is an animation, information about the animation"""
    audio: Audio | None = None
    """*Optional*. Media is an audio file, information about the file; currently, can't be received in a poll option"""
    document: Document | None = None
    """*Optional*. Media is a general file, information about the file; currently, can't be received in a poll option"""
    live_photo: LivePhoto | None = None
    """*Optional*. Media is a live photo, information about the live photo"""
    location: Location | None = None
    """*Optional*. Media is a shared location, information about the location"""
    photo: list[PhotoSize] | None = None
    """*Optional*. Media is a photo, available sizes of the photo"""
    sticker: Sticker | None = None
    """*Optional*. Media is a sticker, information about the sticker; currently, for poll options only"""
    venue: Venue | None = None
    """*Optional*. Media is a venue, information about the venue"""
    video: Video | None = None
    """*Optional*. Media is a video, information about the video"""
    link: Link | None = None
    """*Optional*. The HTTP link attached to the poll option"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            animation: Animation | None = None,
            audio: Audio | None = None,
            document: Document | None = None,
            live_photo: LivePhoto | None = None,
            location: Location | None = None,
            photo: list[PhotoSize] | None = None,
            sticker: Sticker | None = None,
            venue: Venue | None = None,
            video: Video | None = None,
            link: Link | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                animation=animation,
                audio=audio,
                document=document,
                live_photo=live_photo,
                location=location,
                photo=photo,
                sticker=sticker,
                venue=venue,
                video=video,
                link=link,
                **__pydantic_kwargs,
            )
