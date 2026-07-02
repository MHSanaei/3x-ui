from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import PassportElementErrorUnion
from .base import TelegramMethod


class SetPassportDataErrors(TelegramMethod[bool]):
    """
    Informs a user that some of the Telegram Passport elements they provided contains errors. The user will not be able to re-submit their Passport to you until the errors are fixed (the contents of the field for which you returned the error must change). Returns :code:`True` on success.
    Use this if the data submitted by the user doesn't satisfy the standards your service requires for any reason. For example, if a birthday date seems invalid, a submitted document is blurry, a scan shows evidence of tampering, etc. Supply some details in the error message to make sure the user knows how to correct the issues.

    Source: https://core.telegram.org/bots/api#setpassportdataerrors
    """

    __returning__ = bool
    __api_method__ = "setPassportDataErrors"

    user_id: int
    """User identifier"""
    errors: list[PassportElementErrorUnion]
    """A JSON-serialized array describing the errors"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            errors: list[PassportElementErrorUnion],
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(user_id=user_id, errors=errors, **__pydantic_kwargs)
