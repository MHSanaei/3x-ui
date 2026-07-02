from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING, Any

from aiogram.utils.dataclass import dataclass_kwargs

if TYPE_CHECKING:
    from aiogram.types import LinkPreviewOptions


# @dataclass ??
class Default:
    # Is not a dataclass because of JSON serialization.

    __slots__ = ("_name",)

    def __init__(self, name: str) -> None:
        self._name = name

    @property
    def name(self) -> str:
        return self._name

    def __str__(self) -> str:
        return f"Default({self._name!r})"

    def __repr__(self) -> str:
        return f"<{self}>"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Default):
            return NotImplemented
        return self._name == other._name

    def __hash__(self) -> int:
        return hash(self._name)


@dataclass(**dataclass_kwargs(slots=True, kw_only=True))
class DefaultBotProperties:
    """
    Default bot properties.
    """

    parse_mode: str | None = None
    """Default parse mode for messages."""
    disable_notification: bool | None = None
    """Sends the message silently. Users will receive a notification with no sound."""
    protect_content: bool | None = None
    """Protects content from copying."""
    allow_sending_without_reply: bool | None = None
    """Allows to send messages without reply."""
    link_preview: LinkPreviewOptions | None = None
    """Link preview settings."""
    link_preview_is_disabled: bool | None = None
    """Disables link preview."""
    link_preview_prefer_small_media: bool | None = None
    """Prefer small media in link preview."""
    link_preview_prefer_large_media: bool | None = None
    """Prefer large media in link preview."""
    link_preview_show_above_text: bool | None = None
    """Show link preview above text."""
    show_caption_above_media: bool | None = None
    """Show caption above media."""

    def __post_init__(self) -> None:
        has_any_link_preview_option = any(
            (
                self.link_preview_is_disabled,
                self.link_preview_prefer_small_media,
                self.link_preview_prefer_large_media,
                self.link_preview_show_above_text,
            ),
        )

        if has_any_link_preview_option and self.link_preview is None:
            from aiogram.types import LinkPreviewOptions

            self.link_preview = LinkPreviewOptions(
                is_disabled=self.link_preview_is_disabled,
                prefer_small_media=self.link_preview_prefer_small_media,
                prefer_large_media=self.link_preview_prefer_large_media,
                show_above_text=self.link_preview_show_above_text,
            )

    def __getitem__(self, item: str) -> Any:
        return getattr(self, item, None)
