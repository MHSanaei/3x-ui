from enum import Enum


class TopicIconColor(int, Enum):
    """
    Color of the topic icon in RGB format.

    Source: https://github.com/telegramdesktop/tdesktop/blob/991fe491c5ae62705d77aa8fdd44a79caf639c45/Telegram/SourceFiles/data/data_forum_topic.cpp#L51-L56
    """

    BLUE = 0x6FB9F0
    YELLOW = 0xFFD67E
    VIOLET = 0xCB86DB
    GREEN = 0x8EEE98
    ROSE = 0xFF93B2
    RED = 0xFB6F5F
