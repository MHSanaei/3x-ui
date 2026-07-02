from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorFiles(PassportElementError):
    """
    Represents an issue with a list of scans. The error is considered resolved when the list of files containing the scans changes.

    Source: https://core.telegram.org/bots/api#passportelementerrorfiles
    """

    source: Literal[PassportElementErrorType.FILES] = PassportElementErrorType.FILES
    """Error source, must be *files*"""
    type: str
    """The section of the user's Telegram Passport which has the issue, one of 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration', 'temporary_registration'"""
    file_hashes: list[str]
    """List of base64-encoded file hashes"""
    message: str
    """Error message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[PassportElementErrorType.FILES] = PassportElementErrorType.FILES,
            type: str,
            file_hashes: list[str],
            message: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                source=source,
                type=type,
                file_hashes=file_hashes,
                message=message,
                **__pydantic_kwargs,
            )
