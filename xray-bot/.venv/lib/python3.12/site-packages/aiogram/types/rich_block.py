from .base import TelegramObject


class RichBlock(TelegramObject):
    """
    This object represents a block in a rich formatted message. Currently, it can be any of the following types:

     - :class:`aiogram.types.rich_block_paragraph.RichBlockParagraph`
     - :class:`aiogram.types.rich_block_section_heading.RichBlockSectionHeading`
     - :class:`aiogram.types.rich_block_preformatted.RichBlockPreformatted`
     - :class:`aiogram.types.rich_block_footer.RichBlockFooter`
     - :class:`aiogram.types.rich_block_divider.RichBlockDivider`
     - :class:`aiogram.types.rich_block_mathematical_expression.RichBlockMathematicalExpression`
     - :class:`aiogram.types.rich_block_anchor.RichBlockAnchor`
     - :class:`aiogram.types.rich_block_list.RichBlockList`
     - :class:`aiogram.types.rich_block_block_quotation.RichBlockBlockQuotation`
     - :class:`aiogram.types.rich_block_pull_quotation.RichBlockPullQuotation`
     - :class:`aiogram.types.rich_block_collage.RichBlockCollage`
     - :class:`aiogram.types.rich_block_slideshow.RichBlockSlideshow`
     - :class:`aiogram.types.rich_block_table.RichBlockTable`
     - :class:`aiogram.types.rich_block_details.RichBlockDetails`
     - :class:`aiogram.types.rich_block_map.RichBlockMap`
     - :class:`aiogram.types.rich_block_animation.RichBlockAnimation`
     - :class:`aiogram.types.rich_block_audio.RichBlockAudio`
     - :class:`aiogram.types.rich_block_photo.RichBlockPhoto`
     - :class:`aiogram.types.rich_block_video.RichBlockVideo`
     - :class:`aiogram.types.rich_block_voice_note.RichBlockVoiceNote`
     - :class:`aiogram.types.rich_block_thinking.RichBlockThinking`

    Source: https://core.telegram.org/bots/api#richblock
    """
