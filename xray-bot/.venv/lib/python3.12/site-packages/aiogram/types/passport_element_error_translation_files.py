from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorTranslationFiles(PassportElementError):
    """
    Represents an issue with the translated version of a document. The error is considered resolved when a file with the document translation change.

    Source: https://core.telegram.org/bots/api#passportelementerrortranslationfiles
    """

    source: Literal[PassportElementErrorType.TRANSLATION_FILES] = (
        PassportElementErrorType.TRANSLATION_FILES
    )
    """Error source, must be *translation_files*"""
    type: str
    """Type of element of the user's Telegram Passport which has the issue, one of 'passport', 'driver_license', 'identity_card', 'internal_passport', 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration', 'temporary_registration'"""
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
            source: Literal[
                PassportElementErrorType.TRANSLATION_FILES
            ] = PassportElementErrorType.TRANSLATION_FILES,
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
