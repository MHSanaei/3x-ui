from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .maybe_inaccessible_message_union import MaybeInaccessibleMessageUnion
    from .message_entity import MessageEntity


class PollOptionAdded(TelegramObject):
    """
    Describes a service message about an option added to a poll.

    Source: https://core.telegram.org/bots/api#polloptionadded
    """

    option_persistent_id: str
    """Unique identifier of the added option"""
    option_text: str
    """Option text"""
    poll_message: MaybeInaccessibleMessageUnion | None = None
    """*Optional*. Message containing the poll to which the option was added, if known. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""
    option_text_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the *option_text*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            option_persistent_id: str,
            option_text: str,
            poll_message: MaybeInaccessibleMessageUnion | None = None,
            option_text_entities: list[MessageEntity] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                option_persistent_id=option_persistent_id,
                option_text=option_text,
                poll_message=poll_message,
                option_text_entities=option_text_entities,
                **__pydantic_kwargs,
            )
