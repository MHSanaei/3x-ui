from __future__ import annotations

from typing import TypeAlias

from .message_origin_channel import MessageOriginChannel
from .message_origin_chat import MessageOriginChat
from .message_origin_hidden_user import MessageOriginHiddenUser
from .message_origin_user import MessageOriginUser

MessageOriginUnion: TypeAlias = (
    MessageOriginUser | MessageOriginHiddenUser | MessageOriginChat | MessageOriginChannel
)
