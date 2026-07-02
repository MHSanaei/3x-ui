from .base import TelegramObject


class RichText(TelegramObject):
    """
    This object represents a rich formatted text. Currently, it can be either a String for plain text, an Array of :class:`aiogram.types.rich_text.RichText`, or any of the following types:

     - :class:`aiogram.types.rich_text_bold.RichTextBold`
     - :class:`aiogram.types.rich_text_italic.RichTextItalic`
     - :class:`aiogram.types.rich_text_underline.RichTextUnderline`
     - :class:`aiogram.types.rich_text_strikethrough.RichTextStrikethrough`
     - :class:`aiogram.types.rich_text_spoiler.RichTextSpoiler`
     - :class:`aiogram.types.rich_text_date_time.RichTextDateTime`
     - :class:`aiogram.types.rich_text_text_mention.RichTextTextMention`
     - :class:`aiogram.types.rich_text_subscript.RichTextSubscript`
     - :class:`aiogram.types.rich_text_superscript.RichTextSuperscript`
     - :class:`aiogram.types.rich_text_marked.RichTextMarked`
     - :class:`aiogram.types.rich_text_code.RichTextCode`
     - :class:`aiogram.types.rich_text_custom_emoji.RichTextCustomEmoji`
     - :class:`aiogram.types.rich_text_mathematical_expression.RichTextMathematicalExpression`
     - :class:`aiogram.types.rich_text_url.RichTextUrl`
     - :class:`aiogram.types.rich_text_email_address.RichTextEmailAddress`
     - :class:`aiogram.types.rich_text_phone_number.RichTextPhoneNumber`
     - :class:`aiogram.types.rich_text_bank_card_number.RichTextBankCardNumber`
     - :class:`aiogram.types.rich_text_mention.RichTextMention`
     - :class:`aiogram.types.rich_text_hashtag.RichTextHashtag`
     - :class:`aiogram.types.rich_text_cashtag.RichTextCashtag`
     - :class:`aiogram.types.rich_text_bot_command.RichTextBotCommand`
     - :class:`aiogram.types.rich_text_anchor.RichTextAnchor`
     - :class:`aiogram.types.rich_text_anchor_link.RichTextAnchorLink`
     - :class:`aiogram.types.rich_text_reference.RichTextReference`
     - :class:`aiogram.types.rich_text_reference_link.RichTextReferenceLink`

    Source: https://core.telegram.org/bots/api#richtext
    """
