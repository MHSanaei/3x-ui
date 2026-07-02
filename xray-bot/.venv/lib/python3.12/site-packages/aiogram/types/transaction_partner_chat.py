from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import TransactionPartnerType
from .transaction_partner import TransactionPartner

if TYPE_CHECKING:
    from .chat import Chat
    from .gift import Gift


class TransactionPartnerChat(TransactionPartner):
    """
    Describes a transaction with a chat.

    Source: https://core.telegram.org/bots/api#transactionpartnerchat
    """

    type: Literal[TransactionPartnerType.CHAT] = TransactionPartnerType.CHAT
    """Type of the transaction partner, always 'chat'"""
    chat: Chat
    """Information about the chat"""
    gift: Gift | None = None
    """*Optional*. The gift sent to the chat by the bot"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[TransactionPartnerType.CHAT] = TransactionPartnerType.CHAT,
            chat: Chat,
            gift: Gift | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, chat=chat, gift=gift, **__pydantic_kwargs)
