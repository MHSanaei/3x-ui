from __future__ import annotations

from typing import TypeAlias

from .passport_element_error_data_field import PassportElementErrorDataField
from .passport_element_error_file import PassportElementErrorFile
from .passport_element_error_files import PassportElementErrorFiles
from .passport_element_error_front_side import PassportElementErrorFrontSide
from .passport_element_error_reverse_side import PassportElementErrorReverseSide
from .passport_element_error_selfie import PassportElementErrorSelfie
from .passport_element_error_translation_file import PassportElementErrorTranslationFile
from .passport_element_error_translation_files import (
    PassportElementErrorTranslationFiles,
)
from .passport_element_error_unspecified import PassportElementErrorUnspecified

PassportElementErrorUnion: TypeAlias = (
    PassportElementErrorDataField
    | PassportElementErrorFrontSide
    | PassportElementErrorReverseSide
    | PassportElementErrorSelfie
    | PassportElementErrorFile
    | PassportElementErrorFiles
    | PassportElementErrorTranslationFile
    | PassportElementErrorTranslationFiles
    | PassportElementErrorUnspecified
)
