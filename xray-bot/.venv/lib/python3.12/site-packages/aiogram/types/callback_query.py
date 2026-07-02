from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import AnswerCallbackQuery
    from .maybe_inaccessible_message_union import MaybeInaccessibleMessageUnion
    from .user import User


class CallbackQuery(TelegramObject):
    """
    This object represents an incoming callback query from a callback button in an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If the button that originated the query was attached to a message sent by the bot, the field *message* will be present. If the button was attached to a message sent via the bot (in `inline mode <https://core.telegram.org/bots/api#inline-mode>`_), the field *inline_message_id* will be present. Exactly one of the fields *data* or *game_short_name* will be present.

     **NOTE:** After the user presses a callback button, Telegram clients will display a progress bar until you call :class:`aiogram.methods.answer_callback_query.AnswerCallbackQuery`. It is, therefore, necessary to react by calling :class:`aiogram.methods.answer_callback_query.AnswerCallbackQuery` even if no notification to the user is needed (e.g., without specifying any of the optional parameters).

    Source: https://core.telegram.org/bots/api#callbackquery
    """

    id: str
    """Unique identifier for this query"""
    from_user: User = Field(..., alias="from")
    """Sender"""
    chat_instance: str
    """Global identifier, uniquely corresponding to the chat to which the message with the callback button was sent. Useful for high scores in :class:`aiogram.methods.games.Games`"""
    message: MaybeInaccessibleMessageUnion | None = None
    """*Optional*. Message sent by the bot with the callback button that originated the query"""
    inline_message_id: str | None = None
    """*Optional*. Identifier of the message sent via the bot in inline mode, that originated the query"""
    data: str | None = None
    """*Optional*. Data associated with the callback button. Be aware that the message originated the query can contain no callback buttons with this data"""
    game_short_name: str | None = None
    """*Optional*. Short name of a `Game <https://core.telegram.org/bots/api#games>`_ to be returned, serves as the unique identifier for the game"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            from_user: User,
            chat_instance: str,
            message: MaybeInaccessibleMessageUnion | None = None,
            inline_message_id: str | None = None,
            data: str | None = None,
            game_short_name: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                from_user=from_user,
                chat_instance=chat_instance,
                message=message,
                inline_message_id=inline_message_id,
                data=data,
                game_short_name=game_short_name,
                **__pydantic_kwargs,
            )

    def answer(
        self,
        text: str | None = None,
        show_alert: bool | None = None,
        url: str | None = None,
        cache_time: int | None = None,
        **kwargs: Any,
    ) -> AnswerCallbackQuery:
        """
        Shortcut for method :class:`aiogram.methods.answer_callback_query.AnswerCallbackQuery`
        will automatically fill method attributes:

        - :code:`callback_query_id`

        Use this method to send answers to callback queries sent from `inline keyboards <https://core.telegram.org/bots/features#inline-keyboards>`_. The answer will be displayed to the user as a notification at the top of the chat screen or as an alert. On success, :code:`True` is returned.

         Alternatively, the user can be redirected to the specified Game URL. For this option to work, you must first create a game for your bot via `@BotFather <https://t.me/botfather>`_ and accept the terms. Otherwise, you may use links like :code:`t.me/your_bot?start=XXXX` that open your bot with a parameter.

        Source: https://core.telegram.org/bots/api#answercallbackquery

        :param text: Text of the notification. If not specified, nothing will be shown to the user, 0-200 characters
        :param show_alert: If :code:`True`, an alert will be shown by the client instead of a notification at the top of the chat screen. Defaults to *false*
        :param url: URL that will be opened by the user's client. If you have created a :class:`aiogram.types.game.Game` and accepted the conditions via `@BotFather <https://t.me/botfather>`_, specify the URL that opens your game - note that this will only work if the query comes from a `https://core.telegram.org/bots/api#inlinekeyboardbutton <https://core.telegram.org/bots/api#inlinekeyboardbutton>`_ *callback_game* button
        :param cache_time: The maximum amount of time in seconds that the result of the callback query may be cached client-side. Telegram apps will support caching starting in version 3.14. Defaults to 0
        :return: instance of method :class:`aiogram.methods.answer_callback_query.AnswerCallbackQuery`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import AnswerCallbackQuery

        return AnswerCallbackQuery(
            callback_query_id=self.id,
            text=text,
            show_alert=show_alert,
            url=url,
            cache_time=cache_time,
            **kwargs,
        ).as_(self._bot)
