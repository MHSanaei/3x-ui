from enum import Enum


class PassportElementErrorType(str, Enum):
    """
    This object represents a passport element error type.

    Source: https://core.telegram.org/bots/api#passportelementerror
    """

    DATA = "data"
    FRONT_SIDE = "front_side"
    REVERSE_SIDE = "reverse_side"
    SELFIE = "selfie"
    FILE = "file"
    FILES = "files"
    TRANSLATION_FILE = "translation_file"
    TRANSLATION_FILES = "translation_files"
    UNSPECIFIED = "unspecified"
