from typing import TypeAlias

from .rich_block_anchor import RichBlockAnchor
from .rich_block_animation import RichBlockAnimation
from .rich_block_audio import RichBlockAudio
from .rich_block_block_quotation import RichBlockBlockQuotation
from .rich_block_collage import RichBlockCollage
from .rich_block_details import RichBlockDetails
from .rich_block_divider import RichBlockDivider
from .rich_block_footer import RichBlockFooter
from .rich_block_list import RichBlockList
from .rich_block_map import RichBlockMap
from .rich_block_mathematical_expression import RichBlockMathematicalExpression
from .rich_block_paragraph import RichBlockParagraph
from .rich_block_photo import RichBlockPhoto
from .rich_block_preformatted import RichBlockPreformatted
from .rich_block_pull_quotation import RichBlockPullQuotation
from .rich_block_section_heading import RichBlockSectionHeading
from .rich_block_slideshow import RichBlockSlideshow
from .rich_block_table import RichBlockTable
from .rich_block_thinking import RichBlockThinking
from .rich_block_video import RichBlockVideo
from .rich_block_voice_note import RichBlockVoiceNote

RichBlockUnion: TypeAlias = (
    RichBlockParagraph
    | RichBlockSectionHeading
    | RichBlockPreformatted
    | RichBlockFooter
    | RichBlockDivider
    | RichBlockMathematicalExpression
    | RichBlockAnchor
    | RichBlockList
    | RichBlockBlockQuotation
    | RichBlockPullQuotation
    | RichBlockCollage
    | RichBlockSlideshow
    | RichBlockTable
    | RichBlockDetails
    | RichBlockMap
    | RichBlockAnimation
    | RichBlockAudio
    | RichBlockPhoto
    | RichBlockVideo
    | RichBlockVoiceNote
    | RichBlockThinking
)
