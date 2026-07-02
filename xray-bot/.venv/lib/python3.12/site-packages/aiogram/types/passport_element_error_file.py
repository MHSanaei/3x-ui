from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorFile(PassportElementError):
    """
    Represents an issue with a document scan. The error is considered resolved when the file with the document scan changes.

    Source: https://core.telegram.org/bots/api#passportelementerrorfile
    """

    source: Literal[PassportElementErrorType.FILE] = PassportElementErrorType.FILE
    """Error source, must be *file*"""
    type: str
    """The section of the user's Telegram Passport which has the issue, one of 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration', 'temporary_registration'"""
    file_hash: str
    """Base64-encoded file hash"""
    message: str
    """Error message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[PassportElementErrorType.FILE] = PassportElementErrorType.FILE,
            type: str,
            file_hash: str,
            message: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                source=source, type=type, file_hash=file_hash, message=message, **__pydantic_kwargs
            )
