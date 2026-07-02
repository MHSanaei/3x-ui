from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from aiogram.enums import StoryAreaTypeType

from .story_area_type import StoryAreaType

if TYPE_CHECKING:
    from .reaction_type_union import ReactionTypeUnion


class StoryAreaTypeSuggestedReaction(StoryAreaType):
    """
    Describes a story area pointing to a suggested reaction. Currently, a story can have up to 5 suggested reaction areas.

    Source: https://core.telegram.org/bots/api#storyareatypesuggestedreaction
    """

    type: Literal[StoryAreaTypeType.SUGGESTED_REACTION] = StoryAreaTypeType.SUGGESTED_REACTION
    """Type of the area, always 'suggested_reaction'"""
    reaction_type: ReactionTypeUnion
    """Type of the reaction"""
    is_dark: bool | None = None
    """*Optional*. Pass :code:`True` if the reaction area has a dark background"""
    is_flipped: bool | None = None
    """*Optional*. Pass :code:`True` if reaction area corner is flipped"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[
                StoryAreaTypeType.SUGGESTED_REACTION
            ] = StoryAreaTypeType.SUGGESTED_REACTION,
            reaction_type: ReactionTypeUnion,
            is_dark: bool | None = None,
            is_flipped: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                reaction_type=reaction_type,
                is_dark=is_dark,
                is_flipped=is_flipped,
                **__pydantic_kwargs,
            )
