from enum import Enum


class RichBlockType(str, Enum):
    """
    This object represents a block in a rich formatted message.

    Source: https://core.telegram.org/bots/api#richtext
    """

    PARAGRAPH = "paragraph"
    HEADING = "heading"
    PRE = "pre"
    FOOTER = "footer"
    DIVIDER = "divider"
    MATHEMATICAL_EXPRESSION = "mathematical_expression"
    ANCHOR = "anchor"
    LIST = "list"
    BLOCKQUOTE = "blockquote"
    PULLQUOTE = "pullquote"
    COLLAGE = "collage"
    SLIDESHOW = "slideshow"
    TABLE = "table"
    DETAILS = "details"
    MAP = "map"
    ANIMATION = "animation"
    AUDIO = "audio"
    PHOTO = "photo"
    VIDEO = "video"
    VOICE_NOTE = "voice_note"
    THINKING = "thinking"
