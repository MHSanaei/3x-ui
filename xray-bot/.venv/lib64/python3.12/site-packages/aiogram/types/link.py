from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class Link(TelegramObject):
    """
    Represents an HTTP link.

    Source: https://core.telegram.org/bots/api#link
    """

    url: str
    """URL of the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, url: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(url=url, **__pydantic_kwargs)
