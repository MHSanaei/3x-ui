from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject


class WriteAccessAllowed(TelegramObject):
    """
    This object represents a service message about a user allowing a bot to write messages after adding it to the attachment menu, launching a Web App from a link, or accepting an explicit request from a Web App sent by the method `requestWriteAccess <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_.

    Source: https://core.telegram.org/bots/api#writeaccessallowed
    """

    from_request: bool | None = None
    """*Optional*. :code:`True`, if the access was granted after the user accepted an explicit request from a Web App sent by the method `requestWriteAccess <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_"""
    web_app_name: str | None = None
    """*Optional*. Name of the Web App, if the access was granted when the Web App was launched from a link"""
    from_attachment_menu: bool | None = None
    """*Optional*. :code:`True`, if the access was granted when the bot was added to the attachment or side menu"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            from_request: bool | None = None,
            web_app_name: str | None = None,
            from_attachment_menu: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                from_request=from_request,
                web_app_name=web_app_name,
                from_attachment_menu=from_attachment_menu,
                **__pydantic_kwargs,
            )
