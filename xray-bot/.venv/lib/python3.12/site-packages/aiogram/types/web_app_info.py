from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject


class WebAppInfo(TelegramObject):
    """
    Describes a `Web App <https://core.telegram.org/bots/webapps>`_.

    Source: https://core.telegram.org/bots/api#webappinfo
    """

    url: str
    """An HTTPS URL of a Web App to be opened with additional data as specified in `Initializing Web Apps <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(__pydantic__self__, *, url: str, **__pydantic_kwargs: Any) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(url=url, **__pydantic_kwargs)
