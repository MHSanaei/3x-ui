from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import ChatMemberStatus
from .chat_member import ChatMember
from .custom import DateTime

if TYPE_CHECKING:
    from .user import User


class ChatMemberRestricted(ChatMember):
    """
    Represents a `chat member <https://core.telegram.org/bots/api#chatmember>`_ that is under certain restrictions in the chat. Supergroups only.

    Source: https://core.telegram.org/bots/api#chatmemberrestricted
    """

    status: Literal[ChatMemberStatus.RESTRICTED] = ChatMemberStatus.RESTRICTED
    """The member's status in the chat, always 'restricted'"""
    user: User
    """Information about the user"""
    is_member: bool
    """:code:`True`, if the user is a member of the chat at the moment of the request"""
    can_send_messages: bool
    """:code:`True`, if the user is allowed to send text messages, rich messages, contacts, giveaways, giveaway winners, invoices, locations and venues"""
    can_send_audios: bool
    """:code:`True`, if the user is allowed to send audios"""
    can_send_documents: bool
    """:code:`True`, if the user is allowed to send documents"""
    can_send_photos: bool
    """:code:`True`, if the user is allowed to send photos"""
    can_send_videos: bool
    """:code:`True`, if the user is allowed to send videos"""
    can_send_video_notes: bool
    """:code:`True`, if the user is allowed to send video notes"""
    can_send_voice_notes: bool
    """:code:`True`, if the user is allowed to send voice notes"""
    can_send_polls: bool
    """:code:`True`, if the user is allowed to send polls and checklists"""
    can_send_other_messages: bool
    """:code:`True`, if the user is allowed to send animations, games, stickers and use inline bots"""
    can_add_web_page_previews: bool
    """:code:`True`, if the user is allowed to add web page previews to their messages"""
    can_react_to_messages: bool
    """:code:`True`, if the user is allowed to react to messages"""
    can_edit_tag: bool
    """:code:`True`, if the user is allowed to edit their own tag"""
    can_change_info: bool
    """:code:`True`, if the user is allowed to change the chat title, photo and other settings"""
    can_invite_users: bool
    """:code:`True`, if the user is allowed to invite new users to the chat"""
    can_pin_messages: bool
    """:code:`True`, if the user is allowed to pin messages"""
    can_manage_topics: bool
    """:code:`True`, if the user is allowed to create forum topics"""
    until_date: DateTime
    """Date when restrictions will be lifted for this user; Unix time. If 0, then the user is restricted forever"""
    tag: str | None = None
    """*Optional*. Tag of the member"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            status: Literal[ChatMemberStatus.RESTRICTED] = ChatMemberStatus.RESTRICTED,
            user: User,
            is_member: bool,
            can_send_messages: bool,
            can_send_audios: bool,
            can_send_documents: bool,
            can_send_photos: bool,
            can_send_videos: bool,
            can_send_video_notes: bool,
            can_send_voice_notes: bool,
            can_send_polls: bool,
            can_send_other_messages: bool,
            can_add_web_page_previews: bool,
            can_react_to_messages: bool,
            can_edit_tag: bool,
            can_change_info: bool,
            can_invite_users: bool,
            can_pin_messages: bool,
            can_manage_topics: bool,
            until_date: DateTime,
            tag: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                status=status,
                user=user,
                is_member=is_member,
                can_send_messages=can_send_messages,
                can_send_audios=can_send_audios,
                can_send_documents=can_send_documents,
                can_send_photos=can_send_photos,
                can_send_videos=can_send_videos,
                can_send_video_notes=can_send_video_notes,
                can_send_voice_notes=can_send_voice_notes,
                can_send_polls=can_send_polls,
                can_send_other_messages=can_send_other_messages,
                can_add_web_page_previews=can_add_web_page_previews,
                can_react_to_messages=can_react_to_messages,
                can_edit_tag=can_edit_tag,
                can_change_info=can_change_info,
                can_invite_users=can_invite_users,
                can_pin_messages=can_pin_messages,
                can_manage_topics=can_manage_topics,
                until_date=until_date,
                tag=tag,
                **__pydantic_kwargs,
            )
