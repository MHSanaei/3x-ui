from typing import TYPE_CHECKING, Any, Literal

from ..enums import ReactionTypeType
from .reaction_type import ReactionType


class ReactionTypeCustomEmoji(ReactionType):
    """
    The reaction is based on a custom emoji.

    Source: https://core.telegram.org/bots/api#reactiontypecustomemoji
    """

    type: Literal[ReactionTypeType.CUSTOM_EMOJI] = ReactionTypeType.CUSTOM_EMOJI
    """Type of the reaction, always 'custom_emoji'"""
    custom_emoji_id: str
    """Custom emoji identifier"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[ReactionTypeType.CUSTOM_EMOJI] = ReactionTypeType.CUSTOM_EMOJI,
            custom_emoji_id: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, custom_emoji_id=custom_emoji_id, **__pydantic_kwargs)
