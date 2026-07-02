from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..client.default import Default
from .base import TelegramObject

if TYPE_CHECKING:
    from .chat_id_union import ChatIdUnion
    from .message_entity import MessageEntity


class ReplyParameters(TelegramObject):
    """
    Describes reply parameters for the message that is being sent.

    Source: https://core.telegram.org/bots/api#replyparameters
    """

    message_id: int
    """Identifier of the message that will be replied to in the current chat, or in the chat *chat_id* if it is specified"""
    chat_id: ChatIdUnion | None = None
    """*Optional*. If the message to be replied to is from a different chat, unique identifier for the chat or username of the bot, supergroup or channel in the format :code:`@username`. Not supported for messages sent on behalf of a business account and messages from channel direct messages chats"""
    allow_sending_without_reply: bool | Default | None = Default("allow_sending_without_reply")
    """*Optional*. Pass :code:`True` if the message should be sent even if the specified message to be replied to is not found. Always :code:`False` for replies in another chat or forum topic. Always :code:`True` for messages sent on behalf of a business account"""
    quote: str | None = None
    """*Optional*. Quoted part of the message to be replied to; 0-1024 characters after entities parsing. The quote must be an exact substring of the message to be replied to, including *bold*, *italic*, *underline*, *strikethrough*, *spoiler*, *custom_emoji*, and *date_time* entities. The message will fail to send if the quote isn't found in the original message"""
    quote_parse_mode: str | Default | None = Default("parse_mode")
    """*Optional*. Mode for parsing entities in the quote. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    quote_entities: list[MessageEntity] | None = None
    """*Optional*. A JSON-serialized list of special entities that appear in the quote. It can be specified instead of *quote_parse_mode*"""
    quote_position: int | None = None
    """*Optional*. Position of the quote in the original message in UTF-16 code units"""
    checklist_task_id: int | None = None
    """*Optional*. Identifier of the specific checklist task to be replied to"""
    poll_option_id: str | None = None
    """*Optional*. Persistent identifier of the specific poll option to be replied to"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            message_id: int,
            chat_id: ChatIdUnion | None = None,
            allow_sending_without_reply: bool | Default | None = Default(
                "allow_sending_without_reply"
            ),
            quote: str | None = None,
            quote_parse_mode: str | Default | None = Default("parse_mode"),
            quote_entities: list[MessageEntity] | None = None,
            quote_position: int | None = None,
            checklist_task_id: int | None = None,
            poll_option_id: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                message_id=message_id,
                chat_id=chat_id,
                allow_sending_without_reply=allow_sending_without_reply,
                quote=quote,
                quote_parse_mode=quote_parse_mode,
                quote_entities=quote_entities,
                quote_position=quote_position,
                checklist_task_id=checklist_task_id,
                poll_option_id=poll_option_id,
                **__pydantic_kwargs,
            )
