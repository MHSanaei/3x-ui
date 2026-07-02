from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject


class ChatPermissions(MutableTelegramObject):
    """
    Describes actions that a non-administrator user is allowed to take in a chat.

    Source: https://core.telegram.org/bots/api#chatpermissions
    """

    can_send_messages: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send text messages, rich messages, contacts, giveaways, giveaway winners, invoices, locations and venues"""
    can_send_audios: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send audios"""
    can_send_documents: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send documents"""
    can_send_photos: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send photos"""
    can_send_videos: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send videos"""
    can_send_video_notes: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send video notes"""
    can_send_voice_notes: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send voice notes"""
    can_send_polls: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send polls and checklists"""
    can_send_other_messages: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to send animations, games, stickers and use inline bots"""
    can_add_web_page_previews: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to add web page previews to their messages"""
    can_react_to_messages: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to react to messages. If omitted, defaults to the value of *can_send_messages*"""
    can_edit_tag: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to edit their own tag. If omitted, defaults to the value of *can_pin_messages*"""
    can_change_info: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to change the chat title, photo and other settings. Ignored in public supergroups"""
    can_invite_users: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to invite new users to the chat"""
    can_pin_messages: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to pin messages. Ignored in public supergroups"""
    can_manage_topics: bool | None = None
    """*Optional*. :code:`True`, if the user is allowed to create forum topics. If omitted defaults to the value of can_pin_messages"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            can_send_messages: bool | None = None,
            can_send_audios: bool | None = None,
            can_send_documents: bool | None = None,
            can_send_photos: bool | None = None,
            can_send_videos: bool | None = None,
            can_send_video_notes: bool | None = None,
            can_send_voice_notes: bool | None = None,
            can_send_polls: bool | None = None,
            can_send_other_messages: bool | None = None,
            can_add_web_page_previews: bool | None = None,
            can_react_to_messages: bool | None = None,
            can_edit_tag: bool | None = None,
            can_change_info: bool | None = None,
            can_invite_users: bool | None = None,
            can_pin_messages: bool | None = None,
            can_manage_topics: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
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
                **__pydantic_kwargs,
            )
