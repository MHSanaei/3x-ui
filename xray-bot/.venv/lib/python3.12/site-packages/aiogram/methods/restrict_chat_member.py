from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, ChatPermissions, DateTimeUnion
from .base import TelegramMethod


class RestrictChatMember(TelegramMethod[bool]):
    """
    Use this method to restrict a user in a supergroup. The bot must be an administrator in the supergroup for this to work and must have the appropriate administrator rights. Pass :code:`True` for all permissions to lift restrictions from a user. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#restrictchatmember
    """

    __returning__ = bool
    __api_method__ = "restrictChatMember"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`"""
    user_id: int
    """Unique identifier of the target user"""
    permissions: ChatPermissions
    """A JSON-serialized object for new user permissions"""
    use_independent_chat_permissions: bool | None = None
    """Pass :code:`True` if chat permissions are set independently. Otherwise, the *can_send_other_messages* and *can_add_web_page_previews* permissions will imply the *can_send_messages*, *can_send_audios*, *can_send_documents*, *can_send_photos*, *can_send_videos*, *can_send_video_notes*, and *can_send_voice_notes* permissions; the *can_send_polls* permission will imply the *can_send_messages* permission"""
    until_date: DateTimeUnion | None = None
    """Date when restrictions will be lifted for the user; Unix time. If user is restricted for more than 366 days or less than 30 seconds from the current time, they are considered to be restricted forever"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
            user_id: int,
            permissions: ChatPermissions,
            use_independent_chat_permissions: bool | None = None,
            until_date: DateTimeUnion | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
                user_id=user_id,
                permissions=permissions,
                use_independent_chat_permissions=use_independent_chat_permissions,
                until_date=until_date,
                **__pydantic_kwargs,
            )
