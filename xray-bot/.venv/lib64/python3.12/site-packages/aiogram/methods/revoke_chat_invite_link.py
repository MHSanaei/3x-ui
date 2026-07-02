from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ChatInviteLink
from .base import TelegramMethod


class RevokeChatInviteLink(TelegramMethod[ChatInviteLink]):
    """
    Use this method to revoke an invite link created by the bot. If the primary link is revoked, a new link is automatically generated. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns the revoked invite link as :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

    Source: https://core.telegram.org/bots/api#revokechatinvitelink
    """

    __returning__ = ChatInviteLink
    __api_method__ = "revokeChatInviteLink"

    chat_id: ChatIdUnion
    """Unique identifier of the target chat or username of the target channel in the format :code:`@username`"""
    invite_link: str
    """The invite link to revoke"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, chat_id: ChatIdUnion, invite_link: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(chat_id=chat_id, invite_link=invite_link, **__pydantic_kwargs)
