from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .user import User


class ChatInviteLink(TelegramObject):
    """
    Represents an invite link for a chat.

    Source: https://core.telegram.org/bots/api#chatinvitelink
    """

    invite_link: str
    """The invite link. If the link was created by another chat administrator, then the second part of the link will be replaced with '…'"""
    creator: User
    """Creator of the link"""
    creates_join_request: bool
    """:code:`True`, if users joining the chat via the link need to be approved by chat administrators"""
    is_primary: bool
    """:code:`True`, if the link is primary"""
    is_revoked: bool
    """:code:`True`, if the link is revoked"""
    name: str | None = None
    """*Optional*. Invite link name"""
    expire_date: DateTime | None = None
    """*Optional*. Point in time (Unix timestamp) when the link will expire or has been expired"""
    member_limit: int | None = None
    """*Optional*. The maximum number of users that can be members of the chat simultaneously after joining the chat via this invite link; 1-99999"""
    pending_join_request_count: int | None = None
    """*Optional*. Number of pending join requests created using this link"""
    subscription_period: int | None = None
    """*Optional*. The number of seconds the subscription will be active for before the next payment"""
    subscription_price: int | None = None
    """*Optional*. The amount of Telegram Stars a user must pay initially and after each subsequent subscription period to be a member of the chat using the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            invite_link: str,
            creator: User,
            creates_join_request: bool,
            is_primary: bool,
            is_revoked: bool,
            name: str | None = None,
            expire_date: DateTime | None = None,
            member_limit: int | None = None,
            pending_join_request_count: int | None = None,
            subscription_period: int | None = None,
            subscription_price: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                invite_link=invite_link,
                creator=creator,
                creates_join_request=creates_join_request,
                is_primary=is_primary,
                is_revoked=is_revoked,
                name=name,
                expire_date=expire_date,
                member_limit=member_limit,
                pending_join_request_count=pending_join_request_count,
                subscription_period=subscription_period,
                subscription_price=subscription_price,
                **__pydantic_kwargs,
            )
