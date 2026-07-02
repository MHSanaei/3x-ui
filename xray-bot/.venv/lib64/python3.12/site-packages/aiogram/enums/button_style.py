from enum import Enum


class ButtonStyle(str, Enum):
    """
    This object represents a button style (inline- or reply-keyboard).

    Sources:
      * https://core.telegram.org/bots/api#inlinekeyboardbutton
      * https://core.telegram.org/bots/api#keyboardbutton
    """

    DANGER = "danger"
    SUCCESS = "success"
    PRIMARY = "primary"
