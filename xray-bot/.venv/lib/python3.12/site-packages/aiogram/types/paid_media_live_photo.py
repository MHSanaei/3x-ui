from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PaidMediaType
from .paid_media import PaidMedia

if TYPE_CHECKING:
    from .live_photo import LivePhoto


class PaidMediaLivePhoto(PaidMedia):
    """
    The paid media is a `live photo <https://core.telegram.org/bots/api#livephoto>`_.

    Source: https://core.telegram.org/bots/api#paidmedialivephoto
    """

    type: Literal[PaidMediaType.LIVE_PHOTO] = PaidMediaType.LIVE_PHOTO
    """Type of the paid media, always 'live_photo'"""
    live_photo: LivePhoto
    """The photo"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[PaidMediaType.LIVE_PHOTO] = PaidMediaType.LIVE_PHOTO,
            live_photo: LivePhoto,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, live_photo=live_photo, **__pydantic_kwargs)
