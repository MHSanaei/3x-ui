from __future__ import annotations

from typing import TypeAlias

from .inaccessible_message import InaccessibleMessage
from .message import Message

MaybeInaccessibleMessageUnion: TypeAlias = Message | InaccessibleMessage
