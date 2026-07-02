from typing import TYPE_CHECKING, Any, Literal

from ..enums import ReactionTypeType
from .reaction_type import ReactionType


class ReactionTypePaid(ReactionType):
    """
    The reaction is paid.

    Source: https://core.telegram.org/bots/api#reactiontypepaid
    """

    type: Literal[ReactionTypeType.PAID] = ReactionTypeType.PAID
    """Type of the reaction, always 'paid'"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[ReactionTypeType.PAID] = ReactionTypeType.PAID,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, **__pydantic_kwargs)
