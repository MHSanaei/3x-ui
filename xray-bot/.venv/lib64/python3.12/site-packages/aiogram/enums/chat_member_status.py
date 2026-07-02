from enum import Enum


class ChatMemberStatus(str, Enum):
    """
    This object represents chat member status.

    Source: https://core.telegram.org/bots/api#chatmember
    """

    CREATOR = "creator"
    ADMINISTRATOR = "administrator"
    MEMBER = "member"
    RESTRICTED = "restricted"
    LEFT = "left"
    KICKED = "kicked"
