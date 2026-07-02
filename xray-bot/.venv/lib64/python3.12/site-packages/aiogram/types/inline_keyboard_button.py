from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import MutableTelegramObject

if TYPE_CHECKING:
    from .callback_game import CallbackGame
    from .copy_text_button import CopyTextButton
    from .login_url import LoginUrl
    from .switch_inline_query_chosen_chat import SwitchInlineQueryChosenChat
    from .web_app_info import WebAppInfo


class InlineKeyboardButton(MutableTelegramObject):
    """
    This object represents one button of an inline keyboard. Exactly one of the fields other than *text*, *icon_custom_emoji_id*, and *style* must be used to specify the type of the button.

    Source: https://core.telegram.org/bots/api#inlinekeyboardbutton
    """

    text: str
    """Label text on the button"""
    icon_custom_emoji_id: str | None = None
    """*Optional*. Unique identifier of the custom emoji shown before the text of the button. Can only be used by bots that purchased additional usernames on `Fragment <https://fragment.com>`_ or in the messages directly sent by the bot to private, group and supergroup chats if the owner of the bot has a Telegram Premium subscription"""
    style: str | None = None
    """*Optional*. Style of the button. Must be one of 'danger' (red), 'success' (green) or 'primary' (blue). If omitted, then an app-specific style is used"""
    url: str | None = None
    """*Optional*. HTTP or tg:// URL to be opened when the button is pressed. Links :code:`tg://user?id=<user_id>` can be used to mention a user by their identifier without using a username, if this is allowed by their privacy settings"""
    callback_data: str | None = None
    """*Optional*. Data to be sent in a `callback query <https://core.telegram.org/bots/api#callbackquery>`_ to the bot when the button is pressed, 1-64 bytes"""
    web_app: WebAppInfo | None = None
    """*Optional*. Description of the `Web App <https://core.telegram.org/bots/webapps>`_ that will be launched when the user presses the button. The Web App will be able to send an arbitrary message on behalf of the user using the method :class:`aiogram.methods.answer_web_app_query.AnswerWebAppQuery`. Available only in private chats between a user and the bot. Not supported for messages sent on behalf of a business account"""
    login_url: LoginUrl | None = None
    """*Optional*. An HTTPS URL used to automatically authorize the user. Can be used as a replacement for the `Telegram Login Widget <https://core.telegram.org/widgets/login>`_"""
    switch_inline_query: str | None = None
    """*Optional*. If set, pressing the button will prompt the user to select one of their chats, open that chat and insert the bot's username and the specified inline query in the input field. May be empty, in which case just the bot's username will be inserted. Not supported for messages sent in channel direct messages chats and on behalf of a business account"""
    switch_inline_query_current_chat: str | None = None
    """*Optional*. If set, pressing the button will insert the bot's username and the specified inline query in the current chat's input field. May be empty, in which case only the bot's username will be inserted"""
    switch_inline_query_chosen_chat: SwitchInlineQueryChosenChat | None = None
    """*Optional*. If set, pressing the button will prompt the user to select one of their chats of the specified type, open that chat and insert the bot's username and the specified inline query in the input field. Not supported for messages sent in channel direct messages chats and on behalf of a business account"""
    copy_text: CopyTextButton | None = None
    """*Optional*. Description of the button that copies the specified text to the clipboard"""
    callback_game: CallbackGame | None = None
    """*Optional*. Description of the game that will be launched when the user presses the button"""
    pay: bool | None = None
    """*Optional*. Specify :code:`True`, to send a `Pay button <https://core.telegram.org/bots/api#payments>`_. Substrings '⭐' and 'XTR' in the buttons's text will be replaced with a Telegram Star icon"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            text: str,
            icon_custom_emoji_id: str | None = None,
            style: str | None = None,
            url: str | None = None,
            callback_data: str | None = None,
            web_app: WebAppInfo | None = None,
            login_url: LoginUrl | None = None,
            switch_inline_query: str | None = None,
            switch_inline_query_current_chat: str | None = None,
            switch_inline_query_chosen_chat: SwitchInlineQueryChosenChat | None = None,
            copy_text: CopyTextButton | None = None,
            callback_game: CallbackGame | None = None,
            pay: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                text=text,
                icon_custom_emoji_id=icon_custom_emoji_id,
                style=style,
                url=url,
                callback_data=callback_data,
                web_app=web_app,
                login_url=login_url,
                switch_inline_query=switch_inline_query,
                switch_inline_query_current_chat=switch_inline_query_current_chat,
                switch_inline_query_chosen_chat=switch_inline_query_chosen_chat,
                copy_text=copy_text,
                callback_game=callback_game,
                pay=pay,
                **__pydantic_kwargs,
            )
