from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ChatInviteLink, DateTimeUnion
from .base import TelegramMethod


class EditChatInviteLink(TelegramMethod[ChatInviteLink]):
    """
    Use this method to edit a non-primary invite link created by the bot. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

    Source: https://core.telegram.org/bots/api#editchatinvitelink
    """

    __returning__ = ChatInviteLink
    __api_method__ = "editChatInviteLink"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target channel in the format :code:`@username`"""
    invite_link: str
    """The invite link to edit"""
    name: str | None = None
    """Invite link name; 0-32 characters"""
    expire_date: DateTimeUnion | None = None
    """Point in time (Unix timestamp) when the link will expire"""
    member_limit: int | None = None
    """The maximum number of users that can be members of the chat simultaneously after joining the chat via this invite link; 1-99999"""
    creates_join_request: bool | None = None
    """:code:`True`, if users joining the chat via the link need to be approved by chat administrators. If :code:`True`, *member_limit* can't be specified"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            invite_link: str,
            name: str | None = None,
            expire_date: DateTimeUnion | None = None,
            member_limit: int | None = None,
            creates_join_request: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                invite_link=invite_link,
                name=name,
                expire_date=expire_date,
                member_limit=member_limit,
                creates_join_request=creates_join_request,
                **__pydantic_kwargs,
            )
