from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorReverseSide(PassportElementError):
    """
    Represents an issue with the reverse side of a document. The error is considered resolved when the file with reverse side of the document changes.

    Source: https://core.telegram.org/bots/api#passportelementerrorreverseside
    """

    source: Literal[PassportElementErrorType.REVERSE_SIDE] = PassportElementErrorType.REVERSE_SIDE
    """Error source, must be *reverse_side*"""
    type: str
    """The section of the user's Telegram Passport which has the issue, one of 'driver_license', 'identity_card'"""
    file_hash: str
    """Base64-encoded hash of the file with the reverse side of the document"""
    message: str
    """Error message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[
                PassportElementErrorType.REVERSE_SIDE
            ] = PassportElementErrorType.REVERSE_SIDE,
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
