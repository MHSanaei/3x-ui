from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PaidMediaType
from .paid_media import PaidMedia

if TYPE_CHECKING:
    from .photo_size import PhotoSize


class PaidMediaPhoto(PaidMedia):
    """
    The paid media is a photo.

    Source: https://core.telegram.org/bots/api#paidmediaphoto
    """

    type: Literal[PaidMediaType.PHOTO] = PaidMediaType.PHOTO
    """Type of the paid media, always 'photo'"""
    photo: list[PhotoSize]
    """The photo"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[PaidMediaType.PHOTO] = PaidMediaType.PHOTO,
            photo: list[PhotoSize],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, photo=photo, **__pydantic_kwargs)
