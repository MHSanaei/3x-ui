from enum import Enum


class MenuButtonType(str, Enum):
    """
    This object represents an type of Menu button

    Source: https://core.telegram.org/bots/api#menubuttondefault
    """

    DEFAULT = "default"
    COMMANDS = "commands"
    WEB_APP = "web_app"
