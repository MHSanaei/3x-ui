from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ChatInviteLink, DateTimeUnion
from .base import TelegramMethod


class CreateChatSubscriptionInviteLink(TelegramMethod[ChatInviteLink]):
    """
    Use this method to create a `subscription invite link <https://telegram.org/blog/superchannels-star-reactions-subscriptions#star-subscriptions>`_ for a channel chat. The bot must have the *can_invite_users* administrator rights. The link can be edited using the method :class:`aiogram.methods.edit_chat_subscription_invite_link.EditChatSubscriptionInviteLink` or revoked using the method :class:`aiogram.methods.revoke_chat_invite_link.RevokeChatInviteLink`. Returns the new invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

    Source: https://core.telegram.org/bots/api#createchatsubscriptioninvitelink
    """

    __returning__ = ChatInviteLink
    __api_method__ = "createChatSubscriptionInviteLink"

    chat_id: ChatIdUnion
    """Unique identifier for the target channel chat or username of the target channel in the format :code:`@username`"""
    subscription_period: DateTimeUnion
    """The number of seconds the subscription will be active for before the next payment. Currently, it must always be 2592000 (30 days)"""
    subscription_price: int
    """The amount of Telegram Stars a user must pay initially and after each subsequent subscription period to be a member of the chat; 1-10000"""
    name: str | None = None
    """Invite link name; 0-32 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            subscription_period: DateTimeUnion,
            subscription_price: int,
            name: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                subscription_period=subscription_period,
                subscription_price=subscription_price,
                name=name,
                **__pydantic_kwargs,
            )
