from enum import Enum


class ParseMode(str, Enum):
    """
    Formatting options

    Source: https://core.telegram.org/bots/api#formatting-options
    """

    MARKDOWN_V2 = "MarkdownV2"
    MARKDOWN = "Markdown"
    HTML = "HTML"
