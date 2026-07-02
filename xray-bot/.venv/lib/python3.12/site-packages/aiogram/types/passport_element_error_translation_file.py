from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorTranslationFile(PassportElementError):
    """
    Represents an issue with one of the files that constitute the translation of a document. The error is considered resolved when the file changes.

    Source: https://core.telegram.org/bots/api#passportelementerrortranslationfile
    """

    source: Literal[PassportElementErrorType.TRANSLATION_FILE] = (
        PassportElementErrorType.TRANSLATION_FILE
    )
    """Error source, must be *translation_file*"""
    type: str
    """Type of element of the user's Telegram Passport which has the issue, one of 'passport', 'driver_license', 'identity_card', 'internal_passport', 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration', 'temporary_registration'"""
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
            source: Literal[
                PassportElementErrorType.TRANSLATION_FILE
            ] = PassportElementErrorType.TRANSLATION_FILE,
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
