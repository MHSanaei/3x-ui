from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ChatInviteLink
from .base import TelegramMethod


class EditChatSubscriptionInviteLink(TelegramMethod[ChatInviteLink]):
    """
    Use this method to edit a subscription invite link created by the bot. The bot must have the *can_invite_users* administrator rights. Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

    Source: https://core.telegram.org/bots/api#editchatsubscriptioninvitelink
    """

    __returning__ = ChatInviteLink
    __api_method__ = "editChatSubscriptionInviteLink"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    invite_link: str
    """The invite link to edit"""
    name: str | None = None
    """Invite link name; 0-32 characters"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            invite_link: str,
            name: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id, invite_link=invite_link, name=name, **__pydantic_kwargs
            )
