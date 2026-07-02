from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import MutableTelegramObject

if TYPE_CHECKING:
    from .keyboard_button_poll_type import KeyboardButtonPollType
    from .keyboard_button_request_chat import KeyboardButtonRequestChat
    from .keyboard_button_request_managed_bot import KeyboardButtonRequestManagedBot
    from .keyboard_button_request_user import KeyboardButtonRequestUser
    from .keyboard_button_request_users import KeyboardButtonRequestUsers
    from .web_app_info import WebAppInfo


class KeyboardButton(MutableTelegramObject):
    """
    This object represents one button of the reply keyboard. At most one of the fields other than *text*, *icon_custom_emoji_id*, and *style* must be used to specify the type of the button. For simple text buttons, *String* can be used instead of this object to specify the button text.

    Source: https://core.telegram.org/bots/api#keyboardbutton
    """

    text: str
    """Text of the button. If none of the fields other than *text*, *icon_custom_emoji_id*, and *style* are used, it will be sent as a message when the button is pressed"""
    icon_custom_emoji_id: str | None = None
    """*Optional*. Unique identifier of the custom emoji shown before the text of the button. Can only be used by bots that purchased additional usernames on `Fragment <https://fragment.com>`_ or in the messages directly sent by the bot to private, group and supergroup chats if the owner of the bot has a Telegram Premium subscription"""
    style: str | None = None
    """*Optional*. Style of the button. Must be one of 'danger' (red), 'success' (green) or 'primary' (blue). If omitted, then an app-specific style is used"""
    request_users: KeyboardButtonRequestUsers | None = None
    """*Optional*. If specified, pressing the button will open a list of suitable users. Identifiers of selected users will be sent to the bot in a 'users_shared' service message. Available in private chats only"""
    request_chat: KeyboardButtonRequestChat | None = None
    """*Optional*. If specified, pressing the button will open a list of suitable chats. Tapping on a chat will send its identifier to the bot in a 'chat_shared' service message. Available in private chats only"""
    request_managed_bot: KeyboardButtonRequestManagedBot | None = None
    """*Optional*. If specified, pressing the button will ask the user to create and share a bot that will be managed by the current bot. Available for bots that enabled management of other bots in the `@BotFather <https://t.me/BotFather>`_ Mini App. Available in private chats only"""
    request_contact: bool | None = None
    """*Optional*. If :code:`True`, the user's phone number will be sent as a contact when the button is pressed. Available in private chats only"""
    request_location: bool | None = None
    """*Optional*. If :code:`True`, the user's current location will be sent when the button is pressed. Available in private chats only"""
    request_poll: KeyboardButtonPollType | None = None
    """*Optional*. If specified, the user will be asked to create a poll and send it to the bot when the button is pressed. Available in private chats only"""
    web_app: WebAppInfo | None = None
    """*Optional*. If specified, the described `Web App <https://core.telegram.org/bots/webapps>`_ will be launched when the button is pressed. The Web App will be able to send a 'web_app_data' service message. Available in private chats only"""
    request_user: KeyboardButtonRequestUser | None = Field(
        None, json_schema_extra={"deprecated": True}
    )
    """*Optional.* If specified, pressing the button will open a list of suitable users. Tapping on any user will send their identifier to the bot in a 'user_shared' service message. Available in private chats only

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: str,
            icon_custom_emoji_id: str | None = None,
            style: str | None = None,
            request_users: KeyboardButtonRequestUsers | None = None,
            request_chat: KeyboardButtonRequestChat | None = None,
            request_managed_bot: KeyboardButtonRequestManagedBot | None = None,
            request_contact: bool | None = None,
            request_location: bool | None = None,
            request_poll: KeyboardButtonPollType | None = None,
            web_app: WebAppInfo | None = None,
            request_user: KeyboardButtonRequestUser | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                text=text,
                icon_custom_emoji_id=icon_custom_emoji_id,
                style=style,
                request_users=request_users,
                request_chat=request_chat,
                request_managed_bot=request_managed_bot,
                request_contact=request_contact,
                request_location=request_location,
                request_poll=request_poll,
                web_app=web_app,
                request_user=request_user,
                **__pydantic_kwargs,
            )
