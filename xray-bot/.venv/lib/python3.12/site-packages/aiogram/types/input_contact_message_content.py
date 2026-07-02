from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .input_message_content import InputMessageContent


class InputContactMessageContent(InputMessageContent):
    """
    Represents the `content <https://core.telegram.org/bots/api#inputmessagecontent>`_ of a contact message to be sent as the result of an inline query.

    Source: https://core.telegram.org/bots/api#inputcontactmessagecontent
    """

    phone_number: str
    """Contact's phone number"""
    first_name: str
    """Contact's first name"""
    last_name: str | None = None
    """*Optional*. Contact's last name"""
    vcard: str | None = None
    """*Optional*. Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            phone_number: str,
            first_name: str,
            last_name: str | None = None,
            vcard: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                phone_number=phone_number,
                first_name=first_name,
                last_name=last_name,
                vcard=vcard,
                **__pydantic_kwargs,
            )
