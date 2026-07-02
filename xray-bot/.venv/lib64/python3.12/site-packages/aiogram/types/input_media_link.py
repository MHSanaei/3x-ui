from typing import TYPE_CHECKING, Any, Literal

from .input_poll_option_media import InputPollOptionMedia


class InputMediaLink(InputPollOptionMedia):
    """
    Represents an HTTP link to be sent.

    Source: https://core.telegram.org/bots/api#inputmedialink
    """

    type: Literal["link"] = "link"
    """Type of the media, must be *link*"""
    url: str
    """HTTP URL of the link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            type: Literal["link"] = "link",
            url: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(type=type, url=url, **__pydantic_kwargs)
