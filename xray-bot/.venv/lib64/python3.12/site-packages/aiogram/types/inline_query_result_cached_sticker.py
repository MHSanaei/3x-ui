from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion


class InlineQueryResultCachedSticker(InlineQueryResult):
    """
    Represents a link to a sticker stored on the Telegram servers. By default, this sticker will be sent by the user. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the sticker.

    Source: https://core.telegram.org/bots/api#inlinequeryresultcachedsticker
    """

    type: Literal[InlineQueryResultType.STICKER] = InlineQueryResultType.STICKER
    """Type of the result, must be *sticker*"""
    id: str
    """Unique identifier for this result, 1-64 bytes"""
    sticker_file_id: str
    """A valid file identifier of the sticker"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the sticker"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.STICKER] = InlineQueryResultType.STICKER,
            id: str,
            sticker_file_id: str,
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
                sticker_file_id=sticker_file_id,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                **__pydantic_kwargs,
            )
