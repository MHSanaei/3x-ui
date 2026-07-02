from __future__ import annotations

from typing import TypeAlias

from .background_type_chat_theme import BackgroundTypeChatTheme
from .background_type_fill import BackgroundTypeFill
from .background_type_pattern import BackgroundTypePattern
from .background_type_wallpaper import BackgroundTypeWallpaper

BackgroundTypeUnion: TypeAlias = (
    BackgroundTypeFill | BackgroundTypeWallpaper | BackgroundTypePattern | BackgroundTypeChatTheme
)
