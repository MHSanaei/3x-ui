from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorDataField(PassportElementError):
    """
    Represents an issue in one of the data fields that was provided by the user. The error is considered resolved when the field's value changes.

    Source: https://core.telegram.org/bots/api#passportelementerrordatafield
    """

    source: Literal[PassportElementErrorType.DATA] = PassportElementErrorType.DATA
    """Error source, must be *data*"""
    type: str
    """The section of the user's Telegram Passport which has the error, one of 'personal_details', 'passport', 'driver_license', 'identity_card', 'internal_passport', 'address'"""
    field_name: str
    """Name of the data field which has the error"""
    data_hash: str
    """Base64-encoded data hash"""
    message: str
    """Error message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[PassportElementErrorType.DATA] = PassportElementErrorType.DATA,
            type: str,
            field_name: str,
            data_hash: str,
            message: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                source=source,
                type=type,
                field_name=field_name,
                data_hash=data_hash,
                message=message,
                **__pydantic_kwargs,
            )
