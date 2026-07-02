from enum import Enum


class InlineQueryResultType(str, Enum):
    """
    Type of inline query result

    Source: https://core.telegram.org/bots/api#inlinequeryresult
    """

    AUDIO = "audio"
    DOCUMENT = "document"
    GIF = "gif"
    MPEG4_GIF = "mpeg4_gif"
    PHOTO = "photo"
    STICKER = "sticker"
    VIDEO = "video"
    VOICE = "voice"
    ARTICLE = "article"
    CONTACT = "contact"
    GAME = "game"
    LOCATION = "location"
    VENUE = "venue"
