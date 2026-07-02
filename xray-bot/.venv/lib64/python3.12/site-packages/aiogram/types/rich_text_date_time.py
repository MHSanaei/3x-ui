from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import RichTextType
from .base import TelegramObject
from .rich_text import RichText

if TYPE_CHECKING:
    from .rich_text import RichText
    from .rich_text_union import RichTextUnion


class RichTextDateTime(RichText):
    """
    Formatted date and time.

    Source: https://core.telegram.org/bots/api#richtextdatetime
    """

    type: Literal[RichTextType.DATE_TIME] = RichTextType.DATE_TIME
    """Type of the rich text, always 'date_time'"""
    text: RichTextUnion
    """The text"""
    unix_time: int
    """The Unix time associated with the entity"""
    date_time_format: str
    """The string that defines the formatting of the date and time. See `date-time entity formatting <https://core.telegram.org/bots/api#date-time-entity-formatting>`_ for more details"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[RichTextType.DATE_TIME] = RichTextType.DATE_TIME,
            text: RichTextUnion,
            unix_time: int,
            date_time_format: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                text=text,
                unix_time=unix_time,
                date_time_format=date_time_format,
                **__pydantic_kwargs,
            )
