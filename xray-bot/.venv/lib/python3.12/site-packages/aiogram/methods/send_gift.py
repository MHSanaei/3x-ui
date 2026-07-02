from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion
from ..types.message_entity import MessageEntity
from .base import TelegramMethod


class SendGift(TelegramMethod[bool]):
    """
    Sends a gift to the given user or channel chat. The gift can't be converted to Telegram Stars by the receiver. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#sendgift
    """

    __returning__ = bool
    __api_method__ = "sendGift"

    gift_id: str
    """Identifier of the gift; limited gifts can't be sent to channel chats"""
    user_id: int | None = None
    """Required if *chat_id* is not specified. Unique identifier of the target user who will receive the gift"""
    chat_id: ChatIdUnion | None = None
    """Required if *user_id* is not specified. Unique identifier for the chat or username of the channel (in the format :code:`@username`) that will receive the gift"""
    pay_for_upgrade: bool | None = None
    """Pass :code:`True` to pay for the gift upgrade from the bot's balance, thereby making the upgrade free for the receiver"""
    text: str | None = None
    """Text that will be shown along with the gift; 0-128 characters"""
    text_parse_mode: str | None = None
    """Mode for parsing entities in the text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored"""
    text_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the gift text. It can be specified instead of *text_parse_mode*. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            gift_id: str,
            user_id: int | None = None,
            chat_id: ChatIdUnion | None = None,
            pay_for_upgrade: bool | None = None,
            text: str | None = None,
            text_parse_mode: str | None = None,
            text_entities: list[MessageEntity] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                gift_id=gift_id,
                user_id=user_id,
                chat_id=chat_id,
                pay_for_upgrade=pay_for_upgrade,
                text=text,
                text_parse_mode=text_parse_mode,
                text_entities=text_entities,
                **__pydantic_kwargs,
            )
