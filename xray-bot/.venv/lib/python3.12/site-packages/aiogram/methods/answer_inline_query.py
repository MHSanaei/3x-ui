from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..types import InlineQueryResultsButton, InlineQueryResultUnion
from .base import TelegramMethod


class AnswerInlineQuery(TelegramMethod[bool]):
    """
    Use this method to send answers to an inline query. On success, :code:`True` is returned.

    No more than **50** results per query are allowed.

    Source: https://core.telegram.org/bots/api#answerinlinequery
    """

    __returning__ = bool
    __api_method__ = "answerInlineQuery"

    inline_query_id: str
    """Unique identifier for the answered query"""
    results: list[InlineQueryResultUnion]
    """A JSON-serialized array of results for the inline query"""
    cache_time: int | None = None
    """The maximum amount of time in seconds that the result of the inline query may be cached on the server. Defaults to 300"""
    is_personal: bool | None = None
    """Pass :code:`True` if results may be cached on the server side only for the user that sent the query. By default, results may be returned to any user who sends the same query"""
    next_offset: str | None = None
    """Pass the offset that a client should send in the next query with the same text to receive more results. Pass an empty string if there are no more results or if you don't support pagination. Offset length can't exceed 64 bytes"""
    button: InlineQueryResultsButton | None = None
    """A JSON-serialized object describing a button to be shown above inline query results"""
    switch_pm_parameter: str | None = Field(None, json_schema_extra={"deprecated": True})
    """`Deep-linking <https://core.telegram.org/bots/features#deep-linking>`_ parameter for the /start message sent to the bot when user presses the switch button. 1-64 characters, only :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed

.. deprecated:: API:6.7
   https://core.telegram.org/bots/api-changelog#april-21-2023"""
    switch_pm_text: str | None = Field(None, json_schema_extra={"deprecated": True})
    """If passed, clients will display a button with specified text that switches the user to a private chat with the bot and sends the bot a start message with the parameter *switch_pm_parameter*

.. deprecated:: API:6.7
   https://core.telegram.org/bots/api-changelog#april-21-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            inline_query_id: str,
            results: list[InlineQueryResultUnion],
            cache_time: int | None = None,
            is_personal: bool | None = None,
            next_offset: str | None = None,
            button: InlineQueryResultsButton | None = None,
            switch_pm_parameter: str | None = None,
            switch_pm_text: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                inline_query_id=inline_query_id,
                results=results,
                cache_time=cache_time,
                is_personal=is_personal,
                next_offset=next_offset,
                button=button,
                switch_pm_parameter=switch_pm_parameter,
                switch_pm_text=switch_pm_text,
                **__pydantic_kwargs,
            )
