from typing import TYPE_CHECKING, Any

from ..types import InputRichMessage
from .base import TelegramMethod


class SendRichMessageDraft(TelegramMethod[bool]):
    """
    Use this method to stream a partial rich message to a user while the message is being generated. Note that the streamed draft is ephemeral and acts as a temporary 30-second preview - once the output is finalized, you **must** call :class:`aiogram.methods.send_rich_message.SendRichMessage` with the complete message to persist it in the user's chat. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#sendrichmessagedraft
    """

    __returning__ = bool
    __api_method__ = "sendRichMessageDraft"

    chat_id: int
    """Unique identifier for the target private chat"""
    draft_id: int
    """Unique identifier of the message draft; must be non-zero. Changes to drafts with the same identifier are animated"""
    rich_message: InputRichMessage
    """The partial message to be streamed"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: int,
            draft_id: int,
            rich_message: InputRichMessage,
            message_thread_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                draft_id=draft_id,
                rich_message=rich_message,
                message_thread_id=message_thread_id,
                **__pydantic_kwargs,
            )
