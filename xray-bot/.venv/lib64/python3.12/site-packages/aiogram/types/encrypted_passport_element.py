from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .passport_file import PassportFile


class EncryptedPassportElement(TelegramObject):
    """
    Describes documents or other Telegram Passport elements shared with the bot by the user.

    Source: https://core.telegram.org/bots/api#encryptedpassportelement
    """

    type: str
    """Element type. One of 'personal_details', 'passport', 'driver_license', 'identity_card', 'internal_passport', 'address', 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration', 'temporary_registration', 'phone_number', 'email'"""
    hash: str
    """Base64-encoded element hash for using in :class:`aiogram.types.passport_element_error_unspecified.PassportElementErrorUnspecified`"""
    data: str | None = None
    """*Optional*. Base64-encoded encrypted Telegram Passport element data provided by the user; available only for 'personal_details', 'passport', 'driver_license', 'identity_card', 'internal_passport' and 'address' types. Can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""
    phone_number: str | None = None
    """*Optional*. User's verified phone number; available only for 'phone_number' type"""
    email: str | None = None
    """*Optional*. User's verified email address; available only for 'email' type"""
    files: list[PassportFile] | None = None
    """*Optional*. Array of encrypted files with documents provided by the user; available only for 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration' and 'temporary_registration' types. Files can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""
    front_side: PassportFile | None = None
    """*Optional*. Encrypted file with the front side of the document, provided by the user; available only for 'passport', 'driver_license', 'identity_card' and 'internal_passport'. The file can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""
    reverse_side: PassportFile | None = None
    """*Optional*. Encrypted file with the reverse side of the document, provided by the user; available only for 'driver_license' and 'identity_card'. The file can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""
    selfie: PassportFile | None = None
    """*Optional*. Encrypted file with the selfie of the user holding a document, provided by the user; available if requested for 'passport', 'driver_license', 'identity_card' and 'internal_passport'. The file can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""
    translation: list[PassportFile] | None = None
    """*Optional*. Array of encrypted files with translated versions of documents provided by the user; available if requested for 'passport', 'driver_license', 'identity_card', 'internal_passport', 'utility_bill', 'bank_statement', 'rental_agreement', 'passport_registration' and 'temporary_registration' types. Files can be decrypted and verified using the accompanying :class:`aiogram.types.encrypted_credentials.EncryptedCredentials`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: str,
            hash: str,
            data: str | None = None,
            phone_number: str | None = None,
            email: str | None = None,
            files: list[PassportFile] | None = None,
            front_side: PassportFile | None = None,
            reverse_side: PassportFile | None = None,
            selfie: PassportFile | None = None,
            translation: list[PassportFile] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                type=type,
                hash=hash,
                data=data,
                phone_number=phone_number,
                email=email,
                files=files,
                front_side=front_side,
                reverse_side=reverse_side,
                selfie=selfie,
                translation=translation,
                **__pydantic_kwargs,
            )
