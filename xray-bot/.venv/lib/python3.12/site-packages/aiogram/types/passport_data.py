from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .encrypted_credentials import EncryptedCredentials
    from .encrypted_passport_element import EncryptedPassportElement


class PassportData(TelegramObject):
    """
    Describes Telegram Passport data shared with the bot by the user.

    Source: https://core.telegram.org/bots/api#passportdata
    """

    data: list[EncryptedPassportElement]
    """Array with information about documents and other Telegram Passport elements that was shared with the bot"""
    credentials: EncryptedCredentials
    """Encrypted credentials required to decrypt the data"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            data: list[EncryptedPassportElement],
            credentials: EncryptedCredentials,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(data=data, credentials=credentials, **__pydantic_kwargs)
