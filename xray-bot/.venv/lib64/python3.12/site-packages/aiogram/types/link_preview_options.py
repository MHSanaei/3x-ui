from typing import TYPE_CHECKING, Any

from ..client.default import Default
from .base import TelegramObject


class LinkPreviewOptions(TelegramObject):
    """
    Describes the options used for link preview generation.

    Source: https://core.telegram.org/bots/api#linkpreviewoptions
    """

    is_disabled: bool | Default | None = Default("link_preview_is_disabled")
    """*Optional*. :code:`True`, if the link preview is disabled"""
    url: str | None = None
    """*Optional*. URL to use for the link preview. If empty, then the first URL found in the message text will be used"""
    prefer_small_media: bool | Default | None = Default("link_preview_prefer_small_media")
    """*Optional*. :code:`True`, if the media in the link preview is supposed to be shrunk; ignored if the URL isn't explicitly specified or media size change isn't supported for the preview"""
    prefer_large_media: bool | Default | None = Default("link_preview_prefer_large_media")
    """*Optional*. :code:`True`, if the media in the link preview is supposed to be enlarged; ignored if the URL isn't explicitly specified or media size change isn't supported for the preview"""
    show_above_text: bool | Default | None = Default("link_preview_show_above_text")
    """*Optional*. :code:`True`, if the link preview must be shown above the message text; otherwise, the link preview will be shown below the message text"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            is_disabled: bool | Default | None = Default("link_preview_is_disabled"),
            url: str | None = None,
            prefer_small_media: bool | Default | None = Default("link_preview_prefer_small_media"),
            prefer_large_media: bool | Default | None = Default("link_preview_prefer_large_media"),
            show_above_text: bool | Default | None = Default("link_preview_show_above_text"),
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                is_disabled=is_disabled,
                url=url,
                prefer_small_media=prefer_small_media,
                prefer_large_media=prefer_large_media,
                show_above_text=show_above_text,
                **__pydantic_kwargs,
            )
