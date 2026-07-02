from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import InlineQueryResultType
from .inline_query_result import InlineQueryResult

if TYPE_CHECKING:
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_message_content_union import InputMessageContentUnion


class InlineQueryResultContact(InlineQueryResult):
    """
    Represents a contact with a phone number. By default, this contact will be sent by the user. Alternatively, you can use *input_message_content* to send a message with the specified content instead of the contact.

    Source: https://core.telegram.org/bots/api#inlinequeryresultcontact
    """

    type: Literal[InlineQueryResultType.CONTACT] = InlineQueryResultType.CONTACT
    """Type of the result, must be *contact*"""
    id: str
    """Unique identifier for this result, 1-64 Bytes"""
    phone_number: str
    """Contact's phone number"""
    first_name: str
    """Contact's first name"""
    last_name: str | None = None
    """*Optional*. Contact's last name"""
    vcard: str | None = None
    """*Optional*. Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message"""
    input_message_content: InputMessageContentUnion | None = None
    """*Optional*. Content of the message to be sent instead of the contact"""
    thumbnail_url: str | None = None
    """*Optional*. Url of the thumbnail for the result"""
    thumbnail_width: int | None = None
    """*Optional*. Thumbnail width"""
    thumbnail_height: int | None = None
    """*Optional*. Thumbnail height"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal[InlineQueryResultType.CONTACT] = InlineQueryResultType.CONTACT,
            id: str,
            phone_number: str,
            first_name: str,
            last_name: str | None = None,
            vcard: str | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            input_message_content: InputMessageContentUnion | None = None,
            thumbnail_url: str | None = None,
            thumbnail_width: int | None = None,
            thumbnail_height: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                id=id,
                phone_number=phone_number,
                first_name=first_name,
                last_name=last_name,
                vcard=vcard,
                reply_markup=reply_markup,
                input_message_content=input_message_content,
                thumbnail_url=thumbnail_url,
                thumbnail_width=thumbnail_width,
                thumbnail_height=thumbnail_height,
                **__pydantic_kwargs,
            )
