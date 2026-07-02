from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_block_caption import RichBlockCaption
    from .voice import Voice


class RichBlockVoiceNote(RichBlock):
    """
    A block with a voice note, corresponding to the HTML tag :code:`<audio>`.

    Source: https://core.telegram.org/bots/api#richblockvoicenote
    """

    type: Literal[RichBlockType.VOICE_NOTE] = RichBlockType.VOICE_NOTE
    """Type of the block, always 'voice_note'"""
    voice_note: Voice
    """The voice note"""
    caption: RichBlockCaption | None = None
    """*Optional*. Caption of the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.VOICE_NOTE] = RichBlockType.VOICE_NOTE,
            voice_note: Voice,
            caption: RichBlockCaption | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type, voice_note=voice_note, caption=caption, **__pydantic_kwargs
            )
