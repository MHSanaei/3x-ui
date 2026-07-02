from __future__ import annotations

from typing import TypeAlias

from .reaction_type_custom_emoji import ReactionTypeCustomEmoji
from .reaction_type_emoji import ReactionTypeEmoji
from .reaction_type_paid import ReactionTypePaid

ReactionTypeUnion: TypeAlias = ReactionTypeEmoji | ReactionTypeCustomEmoji | ReactionTypePaid
