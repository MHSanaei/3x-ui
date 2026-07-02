from __future__ import annotations

from typing import TYPE_CHECKING, TypeAlias

from typing_extensions import TypeAliasType

from .rich_text_anchor import RichTextAnchor
from .rich_text_anchor_link import RichTextAnchorLink
from .rich_text_bank_card_number import RichTextBankCardNumber
from .rich_text_bold import RichTextBold
from .rich_text_bot_command import RichTextBotCommand
from .rich_text_cashtag import RichTextCashtag
from .rich_text_code import RichTextCode
from .rich_text_custom_emoji import RichTextCustomEmoji
from .rich_text_date_time import RichTextDateTime
from .rich_text_email_address import RichTextEmailAddress
from .rich_text_hashtag import RichTextHashtag
from .rich_text_italic import RichTextItalic
from .rich_text_marked import RichTextMarked
from .rich_text_mathematical_expression import RichTextMathematicalExpression
from .rich_text_mention import RichTextMention
from .rich_text_phone_number import RichTextPhoneNumber
from .rich_text_reference import RichTextReference
from .rich_text_reference_link import RichTextReferenceLink
from .rich_text_spoiler import RichTextSpoiler
from .rich_text_strikethrough import RichTextStrikethrough
from .rich_text_subscript import RichTextSubscript
from .rich_text_superscript import RichTextSuperscript
from .rich_text_text_mention import RichTextTextMention
from .rich_text_underline import RichTextUnderline
from .rich_text_url import RichTextUrl

if TYPE_CHECKING:
    RichTextUnion: TypeAlias = (
        str
        | list["RichTextUnion"]
        | RichTextBold
        | RichTextItalic
        | RichTextUnderline
        | RichTextStrikethrough
        | RichTextSpoiler
        | RichTextDateTime
        | RichTextTextMention
        | RichTextSubscript
        | RichTextSuperscript
        | RichTextMarked
        | RichTextCode
        | RichTextCustomEmoji
        | RichTextMathematicalExpression
        | RichTextUrl
        | RichTextEmailAddress
        | RichTextPhoneNumber
        | RichTextBankCardNumber
        | RichTextMention
        | RichTextHashtag
        | RichTextCashtag
        | RichTextBotCommand
        | RichTextAnchor
        | RichTextAnchorLink
        | RichTextReference
        | RichTextReferenceLink
    )
else:
    RichTextUnion = TypeAliasType(
        "RichTextUnion",
        str
        | list["RichTextUnion"]
        | RichTextBold
        | RichTextItalic
        | RichTextUnderline
        | RichTextStrikethrough
        | RichTextSpoiler
        | RichTextDateTime
        | RichTextTextMention
        | RichTextSubscript
        | RichTextSuperscript
        | RichTextMarked
        | RichTextCode
        | RichTextCustomEmoji
        | RichTextMathematicalExpression
        | RichTextUrl
        | RichTextEmailAddress
        | RichTextPhoneNumber
        | RichTextBankCardNumber
        | RichTextMention
        | RichTextHashtag
        | RichTextCashtag
        | RichTextBotCommand
        | RichTextAnchor
        | RichTextAnchorLink
        | RichTextReference
        | RichTextReferenceLink,
    )
