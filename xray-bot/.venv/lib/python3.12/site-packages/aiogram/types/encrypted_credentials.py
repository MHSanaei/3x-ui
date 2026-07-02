from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class EncryptedCredentials(TelegramObject):
    """
    Describes data required for decrypting and authenticating :class:`aiogram.types.encrypted_passport_element.EncryptedPassportElement`. See the `Telegram Passport Documentation <https://core.telegram.org/passport#receiving-information>`_ for a complete description of the data decryption and authentication processes.

    Source: https://core.telegram.org/bots/api#encryptedcredentials
    """

    data: str
    """Base64-encoded encrypted JSON-serialized data with unique user's payload, data hashes and secrets required for :class:`aiogram.types.encrypted_passport_element.EncryptedPassportElement` decryption and authentication"""
    hash: str
    """Base64-encoded data hash for data authentication"""
    secret: str
    """Base64-encoded secret, encrypted with the bot's public RSA key, required for data decryption"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__, *, data: str, hash: str, secret: str, **__pydantic_kwargs: Any
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(data=data, hash=hash, secret=secret, **__pydantic_kwargs)
