from enum import Enum


class RichTextType(str, Enum):
    """
    This object represents the rich text type.

    Source: https://core.telegram.org/bots/api#richtext
    """

    BOLD = "bold"
    ITALIC = "italic"
    UNDERLINE = "underline"
    STRIKETHROUGH = "strikethrough"
    SPOILER = "spoiler"
    DATE_TIME = "date_time"
    TEXT_MENTION = "text_mention"
    SUBSCRIPT = "subscript"
    SUPERSCRIPT = "superscript"
    MARKED = "marked"
    CODE = "code"
    CUSTOM_EMOJI = "custom_emoji"
    MATHEMATICAL_EXPRESSION = "mathematical_expression"
    URL = "url"
    EMAIL_ADDRESS = "email_address"
    PHONE_NUMBER = "phone_number"
    BANK_CARD_NUMBER = "bank_card_number"
    MENTION = "mention"
    HASHTAG = "hashtag"
    CASHTAG = "cashtag"
    BOT_COMMAND = "bot_command"
    ANCHOR = "anchor"
    ANCHOR_LINK = "anchor_link"
    REFERENCE = "reference"
    REFERENCE_LINK = "reference_link"
