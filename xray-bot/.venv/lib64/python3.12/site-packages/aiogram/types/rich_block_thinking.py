from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichBlockType
from .base import TelegramObject
from .rich_block import RichBlock

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichBlockThinking(RichBlock):
    """
    A block with a 'Thinking…' placeholder, corresponding to the custom HTML tag :code:`<tg-thinking>`. The block may be used only in :class:`aiogram.methods.send_rich_message_draft.SendRichMessageDraft`, therefore it can't be received in messages. See `https://t.me/addemoji/AIActions <https://t.me/addemoji/AIActions>`_`https://t.me/addemoji/AIActions <https://t.me/addemoji/AIActions>`_ for examples of custom emoji, which are recommended for usage in the block.

    Source: https://core.telegram.org/bots/api#richblockthinking
    """

    type: Literal[RichBlockType.THINKING] = RichBlockType.THINKING
    """Type of the block, always 'thinking'"""
    text: RichTextUnion
    """Text of the block. See `https://t.me/addemoji/AIActions <https://t.me/addemoji/AIActions>`_`https://t.me/addemoji/AIActions <https://t.me/addemoji/AIActions>`_ for examples of custom emoji, which are recommended for usage in the block"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichBlockType.THINKING] = RichBlockType.THINKING,
            text: RichTextUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, text=text, **__pydantic_kwargs)
