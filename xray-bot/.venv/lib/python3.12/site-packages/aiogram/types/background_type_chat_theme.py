from typing import TYPE_CHECKING, Any, Literal

from .background_type import BackgroundType


class BackgroundTypeChatTheme(BackgroundType):
    """
    The background is taken directly from a built-in chat theme.

    Source: https://core.telegram.org/bots/api#backgroundtypechattheme
    """

    type: Literal["chat_theme"] = "chat_theme"
    """Type of the background, always 'chat_theme'"""
    theme_name: str
    """Name of the chat theme, which is usually an emoji"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["chat_theme"] = "chat_theme",
            theme_name: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, theme_name=theme_name, **__pydantic_kwargs)
