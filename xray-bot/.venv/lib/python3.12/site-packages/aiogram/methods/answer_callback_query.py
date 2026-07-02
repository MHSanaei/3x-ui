from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class AnswerCallbackQuery(TelegramMethod[bool]):
    """
    Use this method to send answers to callback queries sent from `inline keyboards <https://core.telegram.org/bots/features#inline-keyboards>`_. The answer will be displayed to the user as a notification at the top of the chat screen or as an alert. On success, :code:`True` is returned.

     Alternatively, the user can be redirected to the specified Game URL. For this option to work, you must first create a game for your bot via `@BotFather <https://t.me/botfather>`_ and accept the terms. Otherwise, you may use links like :code:`t.me/your_bot?start=XXXX` that open your bot with a parameter.

    Source: https://core.telegram.org/bots/api#answercallbackquery
    """

    __returning__ = bool
    __api_method__ = "answerCallbackQuery"

    callback_query_id: str
    """Unique identifier for the query to be answered"""
    text: str | None = None
    """Text of the notification. If not specified, nothing will be shown to the user, 0-200 characters"""
    show_alert: bool | None = None
    """If :code:`True`, an alert will be shown by the client instead of a notification at the top of the chat screen. Defaults to *false*"""
    url: str | None = None
    """URL that will be opened by the user's client. If you have created a :class:`aiogram.types.game.Game` and accepted the conditions via `@BotFather <https://t.me/botfather>`_, specify the URL that opens your game - note that this will only work if the query comes from a `https://core.telegram.org/bots/api#inlinekeyboardbutton <https://core.telegram.org/bots/api#inlinekeyboardbutton>`_ *callback_game* button"""
    cache_time: int | None = None
    """The maximum amount of time in seconds that the result of the callback query may be cached client-side. Telegram apps will support caching starting in version 3.14. Defaults to 0"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            callback_query_id: str,
            text: str | None = None,
            show_alert: bool | None = None,
            url: str | None = None,
            cache_time: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                callback_query_id=callback_query_id,
                text=text,
                show_alert=show_alert,
                url=url,
                cache_time=cache_time,
                **__pydantic_kwargs,
            )
