from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message


class SuggestedPostDeclined(TelegramObject):
    """
    Describes a service message about the rejection of a suggested post.

    Source: https://core.telegram.org/bots/api#suggestedpostdeclined
    """

    suggested_post_message: Message | None = None
    """*Optional*. Message containing the suggested post. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""
    comment: str | None = None
    """*Optional*. Comment with which the post was declined"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            suggested_post_message: Message | None = None,
            comment: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                suggested_post_message=suggested_post_message, comment=comment, **__pydantic_kwargs
            )
