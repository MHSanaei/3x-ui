from typing import Annotated

from pydantic import Field, TypeAdapter

from aiogram.types import (
    ChatMember,
    ChatMemberAdministrator,
    ChatMemberBanned,
    ChatMemberLeft,
    ChatMemberMember,
    ChatMemberOwner,
    ChatMemberRestricted,
)

ChatMemberUnion = (
    ChatMemberOwner
    | ChatMemberAdministrator
    | ChatMemberMember
    | ChatMemberRestricted
    | ChatMemberLeft
    | ChatMemberBanned
)

ChatMemberCollection = tuple[type[ChatMember], ...]

ChatMemberAdapter: TypeAdapter[ChatMemberUnion] = TypeAdapter(
    Annotated[
        ChatMemberUnion,
        Field(discriminator="status"),
    ],
)

ADMINS: ChatMemberCollection = (ChatMemberOwner, ChatMemberAdministrator)
USERS: ChatMemberCollection = (ChatMemberMember, ChatMemberRestricted)
MEMBERS: ChatMemberCollection = ADMINS + USERS
NOT_MEMBERS: ChatMemberCollection = (ChatMemberLeft, ChatMemberBanned)
