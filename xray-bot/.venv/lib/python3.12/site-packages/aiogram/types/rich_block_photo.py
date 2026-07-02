from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .photo_size import PhotoSize
    from .rich_block_caption import RichBlockCaption


class RichBlockPhoto(RichBlock):
    """
    A block with a photo, corresponding to the HTML tag :code:`<photo>`.

    Source: https://core.telegram.org/bots/api#richblockphoto
    """

    type: Literal[RichBlockType.PHOTO] = RichBlockType.PHOTO
    """Type of the block, always 'photo'"""
    photo: list[PhotoSize]
    """Available sizes of the photo"""
    has_spoiler: bool | None = None
    """*Optional*. :code:`True`, if the media preview is covered by a spoiler animation"""
    caption: RichBlockCaption | None = None
    """*Optional*. Caption of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.PHOTO] = RichBlockType.PHOTO,
            photo: list[PhotoSize],
            has_spoiler: bool | None = None,
            caption: RichBlockCaption | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                photo=photo,
                has_spoiler=has_spoiler,
                caption=caption,
                **__pydantic_kwargs,
            )
