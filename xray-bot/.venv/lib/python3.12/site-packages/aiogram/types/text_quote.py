from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message_entity import MessageEntity


class TextQuote(TelegramObject):
    """
    This object contains information about the quoted part of a message that is replied to by the given message.

    Source: https://core.telegram.org/bots/api#textquote
    """

    text: str
    """Text of the quoted part of a message that is replied to by the given message"""
    position: int
    """Approximate quote position in the original message in UTF-16 code units as specified by the sender"""
    entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the quote. Currently, only *bold*, *italic*, *underline*, *strikethrough*, *spoiler*, *custom_emoji*, and *date_time* entities are kept in quotes"""
    is_manual: bool | None = None
    """*Optional*. :code:`True`, if the quote was chosen manually by the message sender. Otherwise, the quote was added automatically by the server"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: str,
            position: int,
            entities: list[MessageEntity] | None = None,
            is_manual: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                text=text,
                position=position,
                entities=entities,
                is_manual=is_manual,
                **__pydantic_kwargs,
            )
