from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InlineQueryResultUnion, PreparedInlineMessage
from .base import TelegramMethod


class SavePreparedInlineMessage(TelegramMethod[PreparedInlineMessage]):
    """
    Stores a message that can be sent by a user of a Mini App. Returns a :class:`aiogram.types.prepared_inline_message.PreparedInlineMessage` object.

    Source: https://core.telegram.org/bots/api#savepreparedinlinemessage
    """

    __returning__ = PreparedInlineMessage
    __api_method__ = "savePreparedInlineMessage"

    user_id: int
    """Unique identifier of the target user that can use the prepared message"""
    result: InlineQueryResultUnion
    """A JSON-serialized object describing the message to be sent"""
    allow_user_chats: bool | None = None
    """Pass :code:`True` if the message can be sent to private chats with users"""
    allow_bot_chats: bool | None = None
    """Pass :code:`True` if the message can be sent to private chats with bots"""
    allow_group_chats: bool | None = None
    """Pass :code:`True` if the message can be sent to group and supergroup chats"""
    allow_channel_chats: bool | None = None
    """Pass :code:`True` if the message can be sent to channel chats"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            result: InlineQueryResultUnion,
            allow_user_chats: bool | None = None,
            allow_bot_chats: bool | None = None,
            allow_group_chats: bool | None = None,
            allow_channel_chats: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                result=result,
                allow_user_chats=allow_user_chats,
                allow_bot_chats=allow_bot_chats,
                allow_group_chats=allow_group_chats,
                allow_channel_chats=allow_channel_chats,
                **__pydantic_kwargs,
            )
