from __future__ import annotations

from typing import TypeAlias

from .chat_member_administrator import ChatMemberAdministrator
from .chat_member_banned import ChatMemberBanned
from .chat_member_left import ChatMemberLeft
from .chat_member_member import ChatMemberMember
from .chat_member_owner import ChatMemberOwner
from .chat_member_restricted import ChatMemberRestricted

ChatMemberUnion: TypeAlias = (
    ChatMemberOwner
    | ChatMemberAdministrator
    | ChatMemberMember
    | ChatMemberRestricted
    | ChatMemberLeft
    | ChatMemberBanned
)
