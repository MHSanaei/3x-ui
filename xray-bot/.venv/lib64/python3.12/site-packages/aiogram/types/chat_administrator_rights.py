from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    pass


class ChatAdministratorRights(TelegramObject):
    """
    Represents the rights of an administrator in a chat.

    Source: https://core.telegram.org/bots/api#chatadministratorrights
    """

    is_anonymous: bool
    """:code:`True`, if the user's presence in the chat is hidden"""
    can_manage_chat: bool
    """:code:`True`, if the administrator can access the chat event log, get boost list, see hidden supergroup and channel members, report spam messages, ignore slow mode, and send messages to the chat without paying Telegram Stars. Implied by any other administrator privilege"""
    can_delete_messages: bool
    """:code:`True`, if the administrator can delete messages of other users"""
    can_manage_video_chats: bool
    """:code:`True`, if the administrator can manage video chats"""
    can_restrict_members: bool
    """:code:`True`, if the administrator can restrict, ban or unban chat members, or access supergroup statistics"""
    can_promote_members: bool
    """:code:`True`, if the administrator can add new administrators with a subset of their own privileges or demote administrators that they have promoted, directly or indirectly (promoted by administrators that were appointed by the user)"""
    can_change_info: bool
    """:code:`True`, if the user is allowed to change the chat title, photo and other settings"""
    can_invite_users: bool
    """:code:`True`, if the user is allowed to invite new users to the chat"""
    can_post_stories: bool
    """:code:`True`, if the administrator can post stories to the chat"""
    can_edit_stories: bool
    """:code:`True`, if the administrator can edit stories posted by other users, post stories to the chat page, pin chat stories, and access the chat's story archive"""
    can_delete_stories: bool
    """:code:`True`, if the administrator can delete stories posted by other users"""
    can_post_messages: bool | None = None
    """*Optional*. :code:`True`, if the administrator can post messages in the channel, approve suggested posts, or access channel statistics; for channels only"""
    can_edit_messages: bool | None = None
    """*Optional*. :code:`True`, if the administrator can edit messages of other users and can pin messages; for channels only"""
    can_pin_messages: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to pin messages; for groups and supergroups only"""
    can_manage_topics: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to create, rename, close, and reopen forum topics; for supergroups only"""
    can_manage_direct_messages: bool | None = None
    """*Optional*. :code:`True`, if the administrator can manage direct messages of the channel and decline suggested posts; for channels only"""
    can_manage_tags: bool | None = None
    """*Optional*. :code:`True`, if the administrator can edit the tags of regular members; for groups and supergroups only. If omitted defaults to the value of can_pin_messages"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            is_anonymous: bool,
            can_manage_chat: bool,
            can_delete_messages: bool,
            can_manage_video_chats: bool,
            can_restrict_members: bool,
            can_promote_members: bool,
            can_change_info: bool,
            can_invite_users: bool,
            can_post_stories: bool,
            can_edit_stories: bool,
            can_delete_stories: bool,
            can_post_messages: bool | None = None,
            can_edit_messages: bool | None = None,
            can_pin_messages: bool | None = None,
            can_manage_topics: bool | None = None,
            can_manage_direct_messages: bool | None = None,
            can_manage_tags: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                is_anonymous=is_anonymous,
                can_manage_chat=can_manage_chat,
                can_delete_messages=can_delete_messages,
                can_manage_video_chats=can_manage_video_chats,
                can_restrict_members=can_restrict_members,
                can_promote_members=can_promote_members,
                can_change_info=can_change_info,
                can_invite_users=can_invite_users,
                can_post_stories=can_post_stories,
                can_edit_stories=can_edit_stories,
                can_delete_stories=can_delete_stories,
                can_post_messages=can_post_messages,
                can_edit_messages=can_edit_messages,
                can_pin_messages=can_pin_messages,
                can_manage_topics=can_manage_topics,
                can_manage_direct_messages=can_manage_direct_messages,
                can_manage_tags=can_manage_tags,
                **__pydantic_kwargs,
            )
