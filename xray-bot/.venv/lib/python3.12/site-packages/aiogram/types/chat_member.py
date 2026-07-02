from __future__ import annotations

from .base import TelegramObject


class ChatMember(TelegramObject):
    """
    This object contains information about one member of a chat. Currently, the following 6 types of chat members are supported:

     - :class:`aiogram.types.chat_member_owner.ChatMemberOwner`
     - :class:`aiogram.types.chat_member_administrator.ChatMemberAdministrator`
     - :class:`aiogram.types.chat_member_member.ChatMemberMember`
     - :class:`aiogram.types.chat_member_restricted.ChatMemberRestricted`
     - :class:`aiogram.types.chat_member_left.ChatMemberLeft`
     - :class:`aiogram.types.chat_member_banned.ChatMemberBanned`

    Source: https://core.telegram.org/bots/api#chatmember
    """
