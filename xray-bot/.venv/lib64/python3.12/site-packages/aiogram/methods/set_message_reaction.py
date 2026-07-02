from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ReactionTypeUnion
from .base import TelegramMethod


class SetMessageReaction(TelegramMethod[bool]):
    """
    Use this method to change the chosen reactions on a message. Service messages of some types can't be reacted to. Automatically forwarded messages from a channel to its discussion group have the same available reactions as messages in the channel. Bots can't use paid reactions. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#setmessagereaction
    """

    __returning__ = bool
    __api_method__ = "setMessageReaction"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_id: int
    """Identifier of the target message. If the message belongs to a media group, the reaction is set to the first non-deleted message in the group instead"""
    reaction: list[ReactionTypeUnion] | None = None
    """A JSON-serialized list of reaction types to set on the message. Currently, as non-premium users, bots can set up to one reaction per message. A custom emoji reaction can be used if it is either already present on the message or explicitly allowed by chat administrators. Paid reactions can't be used by bots"""
    is_big: bool | None = None
    """Pass :code:`True` to set the reaction with a big animation"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            message_id: int,
            reaction: list[ReactionTypeUnion] | None = None,
            is_big: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                message_id=message_id,
                reaction=reaction,
                is_big=is_big,
                **__pydantic_kwargs,
            )
