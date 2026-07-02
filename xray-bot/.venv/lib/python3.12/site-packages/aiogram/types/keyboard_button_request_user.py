from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject


class KeyboardButtonRequestUser(TelegramObject):
    """
    This object defines the criteria used to request a suitable user. The identifier of the selected user will be shared with the bot when the corresponding button is pressed. `More about requesting users » <https://core.telegram.org/bots/features#chat-and-user-selection>`_

    .. deprecated:: API:7.0
       https://core.telegram.org/bots/api-changelog#december-29-2023

    Source: https://core.telegram.org/bots/api#keyboardbuttonrequestuser
    """

    request_id: int
    """Signed 32-bit identifier of the request, which will be received back in the :class:`aiogram.types.user_shared.UserShared` object. Must be unique within the message"""
    user_is_bot: bool | None = None
    """*Optional*. Pass :code:`True` to request a bot, pass :code:`False` to request a regular user. If not specified, no additional restrictions are applied"""
    user_is_premium: bool | None = None
    """*Optional*. Pass :code:`True` to request a premium user, pass :code:`False` to request a non-premium user. If not specified, no additional restrictions are applied"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            request_id: int,
            user_is_bot: bool | None = None,
            user_is_premium: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                request_id=request_id,
                user_is_bot=user_is_bot,
                user_is_premium=user_is_premium,
                **__pydantic_kwargs,
            )
