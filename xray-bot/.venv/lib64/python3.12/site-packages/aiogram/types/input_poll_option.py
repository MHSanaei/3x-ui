from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..client.default import Default
from .base import TelegramObject

if TYPE_CHECKING:
    from .input_poll_option_media import InputPollOptionMedia
    from .input_poll_option_media_union import InputPollOptionMediaUnion
    from .message_entity import MessageEntity


class InputPollOption(TelegramObject):
    """
    This object contains information about one answer option in a poll to be sent.

    Source: https://core.telegram.org/bots/api#inputpolloption
    """

    text: str
    """Option text, 1-100 characters"""
    text_parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed"""
    text_entities: list[MessageEntity] | None = None
    """*Optional*. A JSON-serialized list of special entities that appear in the poll option text. It can be specified instead of *text_parse_mode*"""
    media: InputPollOptionMediaUnion | None = None
    """*Optional*. Media added to the poll option"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: str,
            text_parse_mode: str | Default | None = Default("parse_mode"),
            text_entities: list[MessageEntity] | None = None,
            media: InputPollOptionMediaUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                text=text,
                text_parse_mode=text_parse_mode,
                text_entities=text_entities,
                media=media,
                **__pydantic_kwargs,
            )
