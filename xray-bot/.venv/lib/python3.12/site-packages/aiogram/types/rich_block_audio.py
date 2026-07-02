from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .audio import Audio
    from .rich_block_caption import RichBlockCaption


class RichBlockAudio(RichBlock):
    """
    A block with a music file, corresponding to the HTML tag :code:`<audio>`.

    Source: https://core.telegram.org/bots/api#richblockaudio
    """

    type: Literal[RichBlockType.AUDIO] = RichBlockType.AUDIO
    """Type of the block, always 'audio'"""
    audio: Audio
    """The audio"""
    caption: RichBlockCaption | None = None
    """*Optional*. Caption of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.AUDIO] = RichBlockType.AUDIO,
            audio: Audio,
            caption: RichBlockCaption | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, audio=audio, caption=caption, **__pydantic_kwargs)
