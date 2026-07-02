from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import AnswerInlineQuery
    from .inline_query_result_union import InlineQueryResultUnion
    from .inline_query_results_button import InlineQueryResultsButton
    from .location import Location
    from .user import User


class InlineQuery(TelegramObject):
    """
    This object represents an incoming inline query. When the user sends an empty query, your bot could return some default or trending results.

    Source: https://core.telegram.org/bots/api#inlinequery
    """

    id: str
    """Unique identifier for this query"""
    from_user: User = Field(..., alias="from")
    """Sender"""
    query: str
    """Text of the query (up to 256 characters)"""
    offset: str
    """Offset of the results to be returned, can be controlled by the bot"""
    chat_type: str | None = None
    """*Optional*. Type of the chat from which the inline query was sent. Can be either 'sender' for a private chat with the inline query sender, 'private', 'group', 'supergroup', or 'channel'. The chat type should be always known for requests sent from official clients and most third-party clients, unless the request was sent from a secret chat"""
    location: Location | None = None
    """*Optional*. Sender location, only for bots that request user location"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            from_user: User,
            query: str,
            offset: str,
            chat_type: str | None = None,
            location: Location | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                from_user=from_user,
                query=query,
                offset=offset,
                chat_type=chat_type,
                location=location,
                **__pydantic_kwargs,
            )

    def answer(
        self,
        results: list[InlineQueryResultUnion],
        cache_time: int | None = None,
        is_personal: bool | None = None,
        next_offset: str | None = None,
        button: InlineQueryResultsButton | None = None,
        switch_pm_parameter: str | None = None,
        switch_pm_text: str | None = None,
        **kwargs: Any,
    ) -> AnswerInlineQuery:
        """
        Shortcut for method :class:`aiogram.methods.answer_inline_query.AnswerInlineQuery`
        will automatically fill method attributes:

        - :code:`inline_query_id`

        Use this method to send answers to an inline query. On success, :code:`True` is returned.

        No more than **50** results per query are allowed.

        Source: https://core.telegram.org/bots/api#answerinlinequery

        :param results: A JSON-serialized array of results for the inline query
        :param cache_time: The maximum amount of time in seconds that the result of the inline query may be cached on the server. Defaults to 300
        :param is_personal: Pass :code:`True` if results may be cached on the server side only for the user that sent the query. By default, results may be returned to any user who sends the same query
        :param next_offset: Pass the offset that a client should send in the next query with the same text to receive more results. Pass an empty string if there are no more results or if you don't support pagination. Offset length can't exceed 64 bytes
        :param button: A JSON-serialized object describing a button to be shown above inline query results
        :param switch_pm_parameter: `Deep-linking <https://core.telegram.org/bots/features#deep-linking>`_ parameter for the /start message sent to the bot when user presses the switch button. 1-64 characters, only :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed
        :param switch_pm_text: If passed, clients will display a button with specified text that switches the user to a private chat with the bot and sends the bot a start message with the parameter *switch_pm_parameter*
        :return: instance of method :class:`aiogram.methods.answer_inline_query.AnswerInlineQuery`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import AnswerInlineQuery

        return AnswerInlineQuery(
            inline_query_id=self.id,
            results=results,
            cache_time=cache_time,
            is_personal=is_personal,
            next_offset=next_offset,
            button=button,
            switch_pm_parameter=switch_pm_parameter,
            switch_pm_text=switch_pm_text,
            **kwargs,
        ).as_(self._bot)
