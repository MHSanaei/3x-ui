from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class UniqueGiftColors(TelegramObject):
    """
    This object contains information about the color scheme for a user's name, message replies and link previews based on a unique gift.

    Source: https://core.telegram.org/bots/api#uniquegiftcolors
    """

    model_custom_emoji_id: str
    """Custom emoji identifier of the unique gift's model"""
    symbol_custom_emoji_id: str
    """Custom emoji identifier of the unique gift's symbol"""
    light_theme_main_color: int
    """Main color used in light themes; RGB format"""
    light_theme_other_colors: list[int]
    """List of 1-3 additional colors used in light themes; RGB format"""
    dark_theme_main_color: int
    """Main color used in dark themes; RGB format"""
    dark_theme_other_colors: list[int]
    """List of 1-3 additional colors used in dark themes; RGB format"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            model_custom_emoji_id: str,
            symbol_custom_emoji_id: str,
            light_theme_main_color: int,
            light_theme_other_colors: list[int],
            dark_theme_main_color: int,
            dark_theme_other_colors: list[int],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                model_custom_emoji_id=model_custom_emoji_id,
                symbol_custom_emoji_id=symbol_custom_emoji_id,
                light_theme_main_color=light_theme_main_color,
                light_theme_other_colors=light_theme_other_colors,
                dark_theme_main_color=dark_theme_main_color,
                dark_theme_other_colors=dark_theme_other_colors,
                **__pydantic_kwargs,
            )
