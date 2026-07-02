from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal

from ..enums import PassportElementErrorType
from .passport_element_error import PassportElementError


class PassportElementErrorUnspecified(PassportElementError):
    """
    Represents an issue in an unspecified place. The error is considered resolved when new data is added.

    Source: https://core.telegram.org/bots/api#passportelementerrorunspecified
    """

    source: Literal[PassportElementErrorType.UNSPECIFIED] = PassportElementErrorType.UNSPECIFIED
    """Error source, must be *unspecified*"""
    type: str
    """Type of element of the user's Telegram Passport which has the issue"""
    element_hash: str
    """Base64-encoded element hash"""
    message: str
    """Error message"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            source: Literal[
                PassportElementErrorType.UNSPECIFIED
            ] = PassportElementErrorType.UNSPECIFIED,
            type: str,
            element_hash: str,
            message: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                source=source,
                type=type,
                element_hash=element_hash,
                message=message,
                **__pydantic_kwargs,
            )
